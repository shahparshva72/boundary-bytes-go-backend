package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const defaultGeminiBaseURL = "https://generativelanguage.googleapis.com/v1beta"

type GeminiSQLConfig struct {
	APIKey  string
	Model   string
	Timeout time.Duration
	BaseURL string
}

type SQLGenerator interface {
	GenerateSQL(ctx context.Context, question string) ([]string, error)
}

type GeminiSQLService struct {
	apiKey     string
	model      string
	baseURL    string
	httpClient *http.Client
}

type SQLResponse struct {
	Queries []string        `json:"queries"`
	Meta    SQLResponseMeta `json:"meta,omitempty"`
}

type SQLResponseMeta struct {
	RequiresSequentialExecution bool   `json:"requiresSequentialExecution"`
	Type                        string `json:"type,omitempty"`
}

func NewGeminiSQLService(config GeminiSQLConfig) *GeminiSQLService {
	timeout := config.Timeout
	if timeout <= 0 {
		timeout = 20 * time.Second
	}

	model := strings.TrimSpace(config.Model)
	if model == "" {
		model = "gemini-2.5-flash"
	}

	baseURL := strings.TrimRight(strings.TrimSpace(config.BaseURL), "/")
	if baseURL == "" {
		baseURL = defaultGeminiBaseURL
	}

	return &GeminiSQLService{
		apiKey:  strings.TrimSpace(config.APIKey),
		model:   model,
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (s *GeminiSQLService) GenerateSQL(ctx context.Context, question string) ([]string, error) {
	if s == nil || s.apiKey == "" {
		return nil, ErrMissingAPIKey
	}

	payload := geminiGenerateRequest{
		SystemInstruction: geminiContent{
			Parts: []geminiPart{{Text: masterPrompt}},
		},
		Contents: []geminiContent{
			{
				Role:  "user",
				Parts: []geminiPart{{Text: question}},
			},
		},
		GenerationConfig: geminiGenerationConfig{
			Temperature:        0.3,
			MaxOutputTokens:    2000,
			ResponseMIMEType:   "application/json",
			ResponseJSONSchema: sqlResponseJSONSchema(),
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("build gemini request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/models/%s:generateContent?key=%s", s.baseURL, url.PathEscape(s.model), url.QueryEscape(s.apiKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("build gemini http request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("gemini request failed: %w", err)
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("read gemini response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, geminiStatusError(resp.StatusCode, responseBody)
	}

	var generated geminiGenerateResponse
	if err := json.Unmarshal(responseBody, &generated); err != nil {
		return nil, fmt.Errorf("decode gemini response: %w", err)
	}

	text := generated.Text()
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("empty response from AI service")
	}

	var sqlResponse SQLResponse
	if err := json.Unmarshal([]byte(text), &sqlResponse); err != nil {
		return nil, fmt.Errorf("decode generated SQL object: %w", err)
	}
	if len(sqlResponse.Queries) == 0 {
		return nil, errors.New("AI response did not include any SQL queries")
	}

	queries := make([]string, 0, len(sqlResponse.Queries))
	for _, query := range sqlResponse.Queries {
		normalized, err := MinimalValidateAndNormalize(query)
		if err != nil {
			return nil, err
		}
		queries = append(queries, normalized)
	}

	return queries, nil
}

var ErrMissingAPIKey = errors.New("missing GOOGLE_GENERATIVE_AI_API_KEY")

type geminiGenerateRequest struct {
	SystemInstruction geminiContent          `json:"systemInstruction,omitempty"`
	Contents          []geminiContent        `json:"contents"`
	GenerationConfig  geminiGenerationConfig `json:"generationConfig"`
}

type geminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []geminiPart `json:"parts"`
}

type geminiPart struct {
	Text string `json:"text"`
}

type geminiGenerationConfig struct {
	Temperature        float64        `json:"temperature"`
	MaxOutputTokens    int            `json:"maxOutputTokens"`
	ResponseMIMEType   string         `json:"responseMimeType"`
	ResponseJSONSchema map[string]any `json:"responseJsonSchema"`
}

type geminiGenerateResponse struct {
	Candidates []struct {
		Content geminiContent `json:"content"`
	} `json:"candidates"`
}

func (r geminiGenerateResponse) Text() string {
	parts := []string{}
	for _, candidate := range r.Candidates {
		for _, part := range candidate.Content.Parts {
			if strings.TrimSpace(part.Text) != "" {
				parts = append(parts, part.Text)
			}
		}
	}
	return strings.Join(parts, "")
}

func geminiStatusError(status int, body []byte) error {
	var payload struct {
		Error struct {
			Message string `json:"message"`
			Status  string `json:"status"`
		} `json:"error"`
	}
	if err := json.Unmarshal(body, &payload); err == nil && payload.Error.Message != "" {
		return fmt.Errorf("gemini API error %d: %s", status, payload.Error.Message)
	}
	return fmt.Errorf("gemini API error %d", status)
}

func sqlResponseJSONSchema() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"queries": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type": "string",
				},
				"minItems": 1,
			},
			"meta": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"requiresSequentialExecution": map[string]any{"type": "boolean"},
					"type": map[string]any{
						"type": "string",
						"enum": []string{"single", "headToHead", "team"},
					},
				},
			},
		},
		"required": []string{"queries"},
	}
}

const masterPrompt = `You are a cricket statistics SQL expert. Convert natural language queries about cricket statistics into safe, accurate PostgreSQL queries for IPL data.

SYSTEM CONTEXT:
- Purpose: Generate accurate, safe PostgreSQL SELECT queries over T20 data for cricket questions.
- Dialect: PostgreSQL 13+.
- Aliases: wpl_delivery AS d, wpl_match AS m, wpl_match_info AS mi, wpl_player AS p.

CURRENT DATE AND RELATIVE TIME:
- Use SQL time functions instead of JavaScript. Always reference CURRENT_DATE in SQL.
- "this year": m.start_date >= DATE_TRUNC('year', CURRENT_DATE)::date AND m.start_date <= CURRENT_DATE.
- "last year": m.start_date >= DATE_TRUNC('year', CURRENT_DATE) - INTERVAL '1 year' AND m.start_date < DATE_TRUNC('year', CURRENT_DATE).
- "last X years": m.start_date >= (DATE_TRUNC('year', CURRENT_DATE) - (INTERVAL '1 year' * X)) AND m.start_date <= CURRENT_DATE.
- "last X months": m.start_date >= (CURRENT_DATE - (INTERVAL '1 month' * X)) AND m.start_date <= CURRENT_DATE.
- Fixed years such as 2018-2020: m.start_date >= '2018-01-01' AND m.start_date <= '2020-12-31'.
- Use m.start_date, never season text, for ordinary date filters.

BBL SEASON HANDLING:
- BBL seasons span calendar years, so use m.season for BBL season phrasing.
- "BBL 2023-24" or "BBL 2024" -> m.season = '2023/24'.
- "BBL 2024-25" or "BBL 2025" -> m.season = '2024/25'.

LEAGUE DETECTION:
- If query mentions IPL or Indian Premier League: m.league = 'IPL'.
- If query mentions WPL or Women's Premier League: m.league = 'WPL'.
- If query mentions BBL, Big Bash League, or Big Bash: m.league = 'BBL'.
- If query mentions WBBL, Women's Big Bash League, or Women's Big Bash: m.league = 'WBBL'.
- If query mentions SA20, SA 20, or South Africa T20: m.league = 'SA20'.
- If query mentions league-specific teams, use that league.
- If no league is explicitly mentioned, default to IPL.

GLOBAL FILTERS:
- Always join deliveries to matches for league and time filters.
- Always exclude Super Overs unless explicitly requested: d.innings <= 2.
- Enforce LIMIT 20 when the user does not request a smaller limit.

SECURITY RULES:
1. Generate ONLY SELECT statements. No INSERT, UPDATE, DELETE, TRUNCATE, ALTER, DROP, CREATE, COPY, EXEC, or transaction statements.
2. Only use these tables: wpl_match m, wpl_delivery d, wpl_match_info mi, wpl_player p.
3. Enforce LIMIT <= 20.
4. No system catalogs, no volatile or dangerous functions.

SCHEMA:
- wpl_match m(match_id, league, season, start_date, venue)
- wpl_delivery d(id, match_id, innings, ball, batting_team, bowling_team, striker, non_striker, bowler, runs_off_bat, extras, wides, noballs, byes, legbyes, penalty, wicket_type, player_dismissed)
- wpl_match_info mi(match_id, league, city, toss_winner, toss_decision, player_of_match, winner)
- wpl_player p(match_id, team_name, player_name)

TEAM NAME NORMALIZATION:
When returning or grouping team names, normalize known variants using a CTE named team_map with variant/canonical columns. Include at least these mappings:
('Royal Challengers Bengaluru','Royal Challengers Bangalore'),
('Delhi Daredevils','Delhi Capitals'),
('Kings XI Punjab','Punjab Kings'),
('Rising Pune Supergiants','Rising Pune Supergiant').
Use COALESCE(tm.canonical, team_field) in SELECT and GROUP BY.

MATCH PHASES:
- Over Number: CAST(SPLIT_PART(d.ball, '.', 1) AS INTEGER) AS over_number.
- Powerplay: over_number BETWEEN 0 AND 5.
- Middle: over_number BETWEEN 6 AND 14.
- Death: over_number BETWEEN 15 AND 19.

BATTING METRICS:
- runs: SUM(d.runs_off_bat)
- balls_faced: COUNT(*) FILTER (WHERE d.wides = 0)
- strike_rate, always include for batting questions: (SUM(d.runs_off_bat)::DECIMAL * 100) / NULLIF(COUNT(*) FILTER (WHERE d.wides = 0), 0) AS strike_rate
- average: SUM(d.runs_off_bat)::DECIMAL / NULLIF(COUNT(CASE WHEN d.player_dismissed = d.striker THEN 1 END), 0)
- boundaries_4: COUNT(*) FILTER (WHERE d.runs_off_bat = 4)
- sixes_6: COUNT(*) FILTER (WHERE d.runs_off_bat = 6)
- dot_balls: COUNT(*) FILTER (WHERE d.runs_off_bat = 0 AND d.extras = 0)
- matches: COUNT(DISTINCT d.match_id)

BOWLING METRICS:
- wickets: COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket'))
- runs_conceded: SUM(d.runs_off_bat + d.wides + d.noballs)
- overs: COUNT(*)::DECIMAL / 6
- economy_rate, always include for bowling questions: SUM(d.runs_off_bat + d.wides + d.noballs) / NULLIF(COUNT(*)::DECIMAL / 6, 0) AS economy_rate
- average: SUM(d.runs_off_bat + d.wides + d.noballs)::DECIMAL / NULLIF(COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL), 0)
- balls_bowled: COUNT(*)
- matches: COUNT(DISTINCT d.match_id)

WINS BY TEAM:
- When returning wins grouped by team from mi.winner, add mi.winner IS NOT NULL in WHERE.
- Use COUNT(*) AS total_wins, not COUNT(mi.winner).
- Normalize winner with team_map.

DUCKS:
Use a batter-innings CTE grouped by match_id, innings, striker to detect runs=0 and dismissed.

PLAYER NAME RESOLUTION:
If a specific player is referenced, generate two queries:
1. Lookup:
SELECT player_name
FROM wpl_player
WHERE player_name ILIKE '%{surname}%'
ORDER BY CASE WHEN player_name ILIKE '{initial}%{surname}' THEN 1 ELSE 2 END
LIMIT 1;
2. Stats query using the literal placeholder 'RESOLVED_PLAYER_NAME'. Do not guess full names.

HEAD-TO-HEAD:
Generate three queries: batter lookup, bowler lookup, final stats query using 'RESOLVED_BATTER_NAME' and 'RESOLVED_BOWLER_NAME'.

COMMON TEMPLATES:
Top scorers:
SELECT d.striker, SUM(d.runs_off_bat) AS runs, COUNT(*) FILTER (WHERE d.wides = 0) AS balls, (SUM(d.runs_off_bat)::DECIMAL * 100) / NULLIF(COUNT(*) FILTER (WHERE d.wides = 0), 0) AS strike_rate
FROM wpl_delivery d
JOIN wpl_match m ON m.match_id = d.match_id
WHERE m.league = 'IPL' AND d.innings <= 2
GROUP BY d.striker
ORDER BY runs DESC
LIMIT 20;

Top wicket takers:
SELECT d.bowler, COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')) AS wickets, SUM(d.runs_off_bat + d.wides + d.noballs) / NULLIF(COUNT(*)::DECIMAL / 6, 0) AS economy_rate
FROM wpl_delivery d
JOIN wpl_match m ON m.match_id = d.match_id
WHERE m.league = 'IPL' AND d.innings <= 2
GROUP BY d.bowler
ORDER BY wickets DESC
LIMIT 20;

OUTPUT CONTRACT:
Return JSON only:
{
  "queries": ["SQL1", "SQL2", "..."],
  "meta": {
    "requiresSequentialExecution": boolean,
    "type": "single|headToHead|team"
  }
}

POST-GENERATION VALIDATION MUST PASS:
- Each SQL is a single SELECT or WITH SELECT.
- Only tables {wpl_match, wpl_delivery, wpl_match_info, wpl_player} appear with allowed aliases {m,d,mi,p}.
- LIMIT exists and is <= 20.
- If batting-oriented, include strike_rate AS strike_rate.
- If bowling-oriented, include economy_rate AS economy_rate.`
