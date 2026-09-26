package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	ErrJevNotConfigured = errors.New("jev service is not configured (missing API key)")
	ErrJevUnavailable   = errors.New("jev service is temporarily unavailable")
)

const (
	defaultJevBaseURL = "https://api.typesafe.ai/v1/systemone"
	defaultJevModel   = "jev-latest"
	defaultJevTimeout = 2 * time.Second

	// Jev is served from us-west-2, so every fresh TCP+TLS handshake costs ~2 extra
	// round trips (~800ms from India). Keep the connection pooled and warm.
	jevIdleConnTimeout  = 5 * time.Minute
	jevKeepWarmInterval = 60 * time.Second
	jevCacheTTL         = 6 * time.Hour
	jevCacheMaxEntries  = 2048
)

type JevConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

type JevQuestion struct {
	Type         string            `json:"type"` // "choice", "score", or "noul"
	Instructions string            `json:"instructions"`
	Choices      []string          `json:"choices,omitempty"`
	Criteria     map[string]string `json:"criteria,omitempty"`
	Levels       []string          `json:"levels,omitempty"`
}

type JevRequest struct {
	Model     string                 `json:"model"`
	State     map[string]interface{} `json:"state"`
	Questions map[string]JevQuestion `json:"questions"`
}

type JevAnswer struct {
	Noul          *float64           `json:"noul,omitempty"`
	Choice        *string            `json:"choice,omitempty"`
	Score         *string            `json:"score,omitempty"`
	Confidence    float64            `json:"confidence"`
	Probabilities map[string]float64 `json:"probabilities,omitempty"`
}

type JevResponse struct {
	Answers map[string]JevAnswer `json:"answers"`
}

type CricketClassification struct {
	IsCricket  bool
	Intent     string
	League     string
	Phase      string
	Confidence float64
	// CricketScore is the raw is_cricket noul value (0 = definitely not cricket).
	CricketScore float64
	RawAnswers   map[string]JevAnswer
}

type JevClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client

	lastUsed atomic.Int64 // unix nanos of the last outbound request

	cacheMu sync.Mutex
	cache   map[string]cachedClassification
}

type cachedClassification struct {
	value     CricketClassification
	expiresAt time.Time
}

func NewJevClient(config JevConfig) *JevClient {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = defaultJevTimeout
	}

	baseURL := strings.TrimSpace(config.BaseURL)
	if baseURL == "" {
		baseURL = defaultJevBaseURL
	}

	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = defaultJevModel
	}

	return &JevClient{
		apiKey:  strings.TrimSpace(config.APIKey),
		baseURL: baseURL,
		model:   model,
		httpClient: &http.Client{
			Timeout:   timeout,
			Transport: newJevTransport(),
		},
		cache: make(map[string]cachedClassification),
	}
}

func newJevTransport() *http.Transport {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.ForceAttemptHTTP2 = true
	transport.MaxIdleConnsPerHost = 16
	transport.IdleConnTimeout = jevIdleConnTimeout
	return transport
}

// Warm opens (or refreshes) a pooled connection to Jev without running a model:
// an empty body is rejected with 422 by request validation, but the TLS session
// stays in the pool so the next real classification pays a single round trip.
func (c *JevClient) Warm(ctx context.Context) error {
	if !c.Configured() {
		return ErrJevNotConfigured
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, strings.NewReader("{}"))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	c.lastUsed.Store(time.Now().UnixNano())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	return resp.Body.Close()
}

// KeepWarm warms the connection immediately and then re-warms it whenever it has
// been idle for longer than the keep-warm interval, until ctx is cancelled.
func (c *JevClient) KeepWarm(ctx context.Context) {
	if !c.Configured() {
		return
	}

	warm := func() {
		warmCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = c.Warm(warmCtx)
	}

	warm()
	ticker := time.NewTicker(jevKeepWarmInterval / 2)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if time.Since(time.Unix(0, c.lastUsed.Load())) >= jevKeepWarmInterval {
				warm()
			}
		}
	}
}

func (c *JevClient) Configured() bool {
	return c != nil && c.apiKey != ""
}

// Evaluate performs a raw System One call to Jev.
func (c *JevClient) Evaluate(ctx context.Context, state map[string]interface{}, questions map[string]JevQuestion) (*JevResponse, error) {
	if !c.Configured() {
		return nil, ErrJevNotConfigured
	}

	reqBody := JevRequest{
		Model:     c.model,
		State:     state,
		Questions: questions,
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("marshal jev request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL, bytes.NewReader(payload))
	if err != nil {
		return nil, fmt.Errorf("build jev http request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")

	c.lastUsed.Store(time.Now().UnixNano())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("%w: %s", ErrJevUnavailable, err.Error())
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read jev response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: status %d (body: %s)", ErrJevUnavailable, resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var jevResp JevResponse
	if err := json.Unmarshal(bodyBytes, &jevResp); err != nil {
		return nil, fmt.Errorf("decode jev response: %w", err)
	}

	return &jevResp, nil
}

// ClassifyCricketQuery executes typed questions specifically for cricket statistics query routing and guardrails.
func (c *JevClient) ClassifyCricketQuery(ctx context.Context, question string) (*CricketClassification, error) {
	cacheKey := strings.ToLower(strings.TrimSpace(question))
	if cached, ok := c.cachedClassification(cacheKey); ok {
		return cached, nil
	}

	questions := map[string]JevQuestion{
		"is_cricket": {
			Type:         "noul",
			Instructions: "Is this question specifically asking about cricket matches, players, teams, leagues, tournaments, rules, or cricket statistics?",
		},
		"intent": {
			Type:         "choice",
			Instructions: "What is the primary cricket question archetype?",
			Criteria: map[string]string{
				"leading_run_scorers":   "Top run scorers, orange cap, or most runs leaderboard",
				"leading_wicket_takers": "Top wicket takers, purple cap, or most wickets leaderboard",
				"most_sixes":            "Most sixes or boundary leaderboard for individual batters",
				"head_to_head":          "Head-to-head matchup between two named players (e.g. Kohli vs Bumrah)",
				"player_batting_stat":   "Batting stats or performance for a specific named player (e.g. 'Kohli runs in IPL', 'Rohit sixes')",
				"player_bowling_stat":   "Bowling stats or performance for a specific named player (e.g. 'Bumrah economy in IPL', 'Shami wickets')",
				"team_record":           "Specific team wins, tournament champions, or team win records",
				"complex_query":         "Leaderboards for strike rate, economy rate, averages, team boundary comparisons, multi-condition queries, or queries without named players",
			},
		},
		"league": {
			Type:         "choice",
			Instructions: "Which cricket tournament or league is referenced, implied, or targeted?",
			Criteria: map[string]string{
				"IPL":     "Indian Premier League",
				"WPL":     "Womens Premier League",
				"BBL":     "Big Bash League",
				"WBBL":    "Womens Big Bash League",
				"SA20":    "SA20 League",
				"unknown": "Unknown or not specified",
			},
		},
		"phase": {
			Type:         "choice",
			Instructions: "Which phase of the match is queried?",
			Criteria: map[string]string{
				"powerplay": "Overs 0 to 5 (Powerplay)",
				"middle":    "Overs 6 to 14 (Middle overs)",
				"death":     "Overs 15 to 19 (Death overs)",
				"all":       "All overs or general match",
			},
		},
	}

	state := map[string]interface{}{
		"question": question,
	}

	resp, err := c.Evaluate(ctx, state, questions)
	if err != nil {
		return nil, err
	}

	isCricket := false
	cricketScore := 0.0
	if isCricketAns, ok := resp.Answers["is_cricket"]; ok && isCricketAns.Noul != nil {
		cricketScore = *isCricketAns.Noul
		isCricket = cricketScore >= 0.70
	}

	intent := "complex_query"
	confidence := 0.0
	if intentAns, ok := resp.Answers["intent"]; ok && intentAns.Choice != nil {
		intent = *intentAns.Choice
		confidence = intentAns.Confidence
	}

	league := "unknown"
	if leagueAns, ok := resp.Answers["league"]; ok && leagueAns.Choice != nil {
		league = *leagueAns.Choice
	}

	phase := "all"
	if phaseAns, ok := resp.Answers["phase"]; ok && phaseAns.Choice != nil {
		phase = *phaseAns.Choice
	}

	classification := &CricketClassification{
		IsCricket:    isCricket,
		Intent:       intent,
		League:       league,
		Phase:        phase,
		Confidence:   confidence,
		CricketScore: cricketScore,
		RawAnswers:   resp.Answers,
	}
	c.storeClassification(cacheKey, classification)
	return classification, nil
}

func (c *JevClient) cachedClassification(key string) (*CricketClassification, bool) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	entry, ok := c.cache[key]
	if !ok {
		return nil, false
	}
	if time.Now().After(entry.expiresAt) {
		delete(c.cache, key)
		return nil, false
	}
	value := entry.value
	return &value, true
}

func (c *JevClient) storeClassification(key string, classification *CricketClassification) {
	c.cacheMu.Lock()
	defer c.cacheMu.Unlock()

	if len(c.cache) >= jevCacheMaxEntries {
		// Classifications are cheap to recompute; a full reset keeps this bounded without LRU bookkeeping.
		clear(c.cache)
	}
	c.cache[key] = cachedClassification{value: *classification, expiresAt: time.Now().Add(jevCacheTTL)}
}
