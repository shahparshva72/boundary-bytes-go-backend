package texttosql

import (
	"context"
	"errors"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/jackc/pgx/v5/pgconn"
	aisql "github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

const (
	CodeValidation = "VALIDATION_ERROR"
	CodeAI         = "AI_ERROR"
	CodeSQL        = "SQL_ERROR"
	CodeDatabase   = "DATABASE_ERROR"
)

var validQuestionPattern = regexp.MustCompile(`^[a-zA-Z0-9\s?.,\-'"()\/:%+&]+$`)
var sanitizePattern = regexp.MustCompile(`[^\w\s?.,\-'"()\/:%+&]`)
var whitespacePattern = regexp.MustCompile(`\s+`)

type SQLGenerator interface {
	GenerateSQL(ctx context.Context, question string) ([]string, error)
}

// SQLRepairer is optionally implemented by generators that can correct SQL the database rejected.
type SQLRepairer interface {
	RepairSQL(ctx context.Context, question string, failedQueries []string, dbError string) ([]string, error)
}

type QueryClassifier interface {
	ClassifyCricketQuery(ctx context.Context, question string) (*aisql.CricketClassification, error)
}

type Repository interface {
	ExecuteAIQuery(ctx context.Context, query string) (models.AIQueryResult, error)
	LogAIRequest(ctx context.Context, params models.LogAIRequestParams) (string, error)
}

type Service struct {
	repository     Repository
	generator      SQLGenerator
	classifier     QueryClassifier
	chat           *aisql.ChatContext
	lookupTimeout  time.Duration
	executeTimeout time.Duration

	resultCacheMu sync.Mutex
	resultCache   map[string]cachedQueryResult
}

// Stats only change when new matches are ingested, so identical SQL can be served
// from memory for a while. Leaderboard aggregates scan ~300k deliveries (~100ms).
const (
	resultCacheTTL        = 10 * time.Minute
	resultCacheMaxEntries = 512
)

type cachedQueryResult struct {
	result    models.AIQueryResult
	expiresAt time.Time
}

type Result struct {
	Data              []map[string]interface{}
	RowCount          int
	ExecutionTimeMS   int
	GeneratedSQL      string
	RequestID         string
	SanitizedQuestion string
	RateLimit         models.RateLimitStatus
	RoutedBy          string
}

type Error struct {
	Code              string
	Message           string
	SanitizedQuestion string
}

func (e *Error) Error() string {
	return e.Message
}

func New(repository Repository, generator SQLGenerator) *Service {
	return &Service{
		repository:     repository,
		generator:      generator,
		lookupTimeout:  15 * time.Second,
		executeTimeout: 20 * time.Second,
		resultCache:    make(map[string]cachedQueryResult),
	}
}

func (s *Service) WithClassifier(classifier QueryClassifier) *Service {
	s.classifier = classifier
	return s
}

// WithChatContext supplies the loaded seasons used to read "<league> <year>".
func (s *Service) WithChatContext(chat *aisql.ChatContext) *Service {
	s.chat = chat
	return s
}

func (s *Service) seasons(ctx context.Context) aisql.SeasonIndex {
	if s.chat == nil {
		return nil
	}
	return s.chat.Seasons(ctx)
}

func (s *Service) Answer(ctx context.Context, question string) (*Result, error) {
	start := time.Now()

	if s.generator == nil {
		return nil, &Error{Code: CodeAI, Message: "AI service configuration error"}
	}

	if errMessage := validateQuestion(question); errMessage != "" {
		s.log(ctx, models.LogAIRequestParams{
			Question:     fallbackQuestion(question),
			Success:      false,
			ErrorCode:    stringPtr(CodeValidation),
			ErrorMessage: &errMessage,
		})
		return nil, &Error{Code: CodeValidation, Message: errMessage}
	}

	sanitizedQuestion := sanitizeQuestion(question)

	routedBy := "gemini"
	var rawGeneratedQueries []string

	if s.classifier != nil {
		classification, err := s.classifier.ClassifyCricketQuery(ctx, sanitizedQuestion)
		if err != nil {
			log.Printf("[Jev] Classification failed: %v (falling back to Gemini)", err)
		} else if classification != nil {
			log.Printf("[Jev] Classified query: intent=%s league=%s phase=%s is_cricket=%t confidence=%.2f",
				classification.Intent, classification.League, classification.Phase, classification.IsCricket, classification.Confidence)

			// Domain Guardrail: reject clearly non-cricket questions without hitting Gemini.
			// Gate on the is_cricket score itself; Confidence belongs to the intent answer.
			if !classification.IsCricket && classification.CricketScore <= 0.30 {
				errMessage := "Please ask a question related to cricket statistics."
				s.log(ctx, models.LogAIRequestParams{
					Question:     fallbackQuestion(question),
					Success:      false,
					ErrorCode:    stringPtr(CodeValidation),
					ErrorMessage: &errMessage,
				})
				return nil, &Error{Code: CodeValidation, Message: errMessage, SanitizedQuestion: sanitizedQuestion}
			}

			// Dynamic Routing: Fast-path via Jev when a known archetype is identified with good confidence (>= 0.75)
			if classification.Confidence >= 0.75 {
				if fastQueries := buildFastPathQueries(classification.Intent, classification.League, classification.Phase, sanitizedQuestion, s.seasons(ctx)); len(fastQueries) > 0 {
					rawGeneratedQueries = fastQueries
					routedBy = "jev"
					log.Printf("[Router] Dynamically routed to fast template (%d queries) for intent=%s league=%s", len(fastQueries), classification.Intent, classification.League)
				}
			}

			if len(rawGeneratedQueries) == 0 {
				log.Printf("[Router] Dynamically delegating to Gemini for complex/novel query (intent=%s)", classification.Intent)
			}
		}
	}

	runQuery := func(sqlText string) ([]map[string]interface{}, error) {
		validation := aisql.ValidateSQL(sqlText)
		if !validation.IsValid {
			return nil, fmt.Errorf("lookup query failed security validation: %s", strings.Join(validation.Errors, ", "))
		}

		queryCtx, cancel := context.WithTimeout(ctx, s.lookupTimeout)
		defer cancel()

		result, err := s.executeCached(queryCtx, sqlText)
		if err != nil {
			return nil, err
		}
		return result.Data, nil
	}

	executePipeline := func(queries []string) (string, []map[string]interface{}, int, error) {
		if len(queries) > 1 {
			if err := aisql.ValidateSequentialQueries(queries); err != nil {
				return "", nil, 0, err
			}
		}

		builtQueries, err := aisql.BuildExecutableQueries(queries, runQuery)
		if err != nil {
			return "", nil, 0, err
		}

		finalSQL := builtQueries[len(builtQueries)-1]
		validation := aisql.ValidateSQL(finalSQL)
		if !validation.IsValid {
			return "", nil, 0, fmt.Errorf("query failed security validation: %s", strings.Join(validation.Errors, ", "))
		}

		queryCtx, cancel := context.WithTimeout(ctx, s.executeTimeout)
		defer cancel()

		queryResult, err := s.executeCached(queryCtx, finalSQL)
		if err != nil {
			return finalSQL, nil, 0, err
		}

		formattedData := queryResult.Data
		if shouldNormalizeTeamResults(finalSQL) {
			formattedData = normalizeTeamResults(formattedData)
		}

		return finalSQL, formattedData, queryResult.ExecutionTimeMS, nil
	}

	var finalSQL string
	var formattedData []map[string]interface{}
	var queryExecTimeMS int

	// If fast-routed by Jev, attempt template execution
	if routedBy == "jev" && len(rawGeneratedQueries) > 0 {
		var execErr error
		finalSQL, formattedData, queryExecTimeMS, execErr = executePipeline(rawGeneratedQueries)
		if execErr != nil {
			log.Printf("[Router] Jev fast template execution failed (%v); gracefully falling back to Gemini full SQL synthesis", execErr)
			rawGeneratedQueries = nil
			routedBy = "gemini"
		}
	}

	// Full synthesis via Gemini (if complex query, low confidence, or template execution failed)
	if len(rawGeneratedQueries) == 0 {
		var err error
		rawGeneratedQueries, err = s.generator.GenerateSQL(ctx, sanitizedQuestion)
		if err != nil {
			return nil, s.recordError(ctx, CodeAI, question, sanitizedQuestion, "", err)
		}

		var execErr error
		finalSQL, formattedData, queryExecTimeMS, execErr = executePipeline(rawGeneratedQueries)

		// One self-correction round when PostgreSQL rejects the generated SQL itself.
		if repairer, ok := s.generator.(SQLRepairer); ok && isRepairableSQLError(execErr) {
			log.Printf("[Gemini] Generated SQL failed (%v); asking for a corrected query", execErr)
			if repaired, repairErr := repairer.RepairSQL(ctx, sanitizedQuestion, rawGeneratedQueries, execErr.Error()); repairErr == nil {
				finalSQL, formattedData, queryExecTimeMS, execErr = executePipeline(repaired)
			} else {
				log.Printf("[Gemini] SQL repair request failed: %v", repairErr)
			}
		}

		if execErr != nil {
			errCode := CodeDatabase
			if strings.Contains(execErr.Error(), "security validation") || strings.Contains(execErr.Error(), "must be SELECT") {
				errCode = CodeSQL
			}
			return nil, s.recordError(ctx, errCode, question, sanitizedQuestion, finalSQL, execErr)
		}
	}

	requestID := s.log(ctx, models.LogAIRequestParams{
		Question:          question,
		SanitizedQuestion: &sanitizedQuestion,
		GeneratedSQL:      &finalSQL,
		RowCount:          intPtr(len(formattedData)),
		ExecutionTimeMS:   &queryExecTimeMS,
		Success:           true,
	})

	return &Result{
		Data:              formattedData,
		RowCount:          len(formattedData),
		ExecutionTimeMS:   int(time.Since(start).Milliseconds()),
		GeneratedSQL:      finalSQL,
		RequestID:         requestID,
		SanitizedQuestion: sanitizedQuestion,
		RoutedBy:          routedBy,
	}, nil
}

// executeCached runs a validated read-only query, reusing a recent result for identical SQL.
// Cached rows are shared between requests and must be treated as read-only.
func (s *Service) executeCached(ctx context.Context, sqlText string) (models.AIQueryResult, error) {
	s.resultCacheMu.Lock()
	entry, ok := s.resultCache[sqlText]
	s.resultCacheMu.Unlock()
	if ok && time.Now().Before(entry.expiresAt) {
		cached := entry.result
		cached.ExecutionTimeMS = 0
		return cached, nil
	}

	result, err := s.repository.ExecuteAIQuery(ctx, sqlText)
	if err != nil {
		return result, err
	}

	s.resultCacheMu.Lock()
	if len(s.resultCache) >= resultCacheMaxEntries {
		clear(s.resultCache)
	}
	s.resultCache[sqlText] = cachedQueryResult{result: result, expiresAt: time.Now().Add(resultCacheTTL)}
	s.resultCacheMu.Unlock()
	return result, nil
}

// isRepairableSQLError reports whether PostgreSQL rejected the query text itself
// (SQLSTATE class 42: syntax errors, unknown columns, GROUP BY violations, ...),
// as opposed to timeouts or connection problems that a rewrite cannot fix.
func isRepairableSQLError(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && strings.HasPrefix(pgErr.Code, "42")
}

func (s *Service) LogInvalidRequest(ctx context.Context, message string) {
	s.log(ctx, models.LogAIRequestParams{
		Question:     "unknown",
		Success:      false,
		ErrorCode:    stringPtr(CodeValidation),
		ErrorMessage: &message,
	})
}

func (s *Service) recordError(ctx context.Context, code, question, sanitizedQuestion, generatedSQL string, err error) *Error {
	errorMessage := err.Error()
	params := models.LogAIRequestParams{
		Question:          question,
		SanitizedQuestion: &sanitizedQuestion,
		Success:           false,
		ErrorCode:         &code,
		ErrorMessage:      &errorMessage,
	}
	if generatedSQL != "" {
		params.GeneratedSQL = &generatedSQL
	}
	s.log(ctx, params)

	return &Error{
		Code:              code,
		Message:           errorMessage,
		SanitizedQuestion: sanitizedQuestion,
	}
}

func (s *Service) log(ctx context.Context, params models.LogAIRequestParams) string {
	id, err := s.repository.LogAIRequest(ctx, params)
	if err != nil {
		return fmt.Sprintf("log-failed-%d", time.Now().UnixMilli())
	}
	return id
}

func validateQuestion(question string) string {
	if question == "" {
		return "Question cannot be empty"
	}
	if len(question) > 500 {
		return "Question is too long"
	}
	if !validQuestionPattern.MatchString(question) {
		return "Question contains invalid characters. Only letters, numbers, spaces, parentheses, and common punctuation are allowed."
	}
	return ""
}

func sanitizeQuestion(question string) string {
	sanitized := strings.TrimSpace(question)
	sanitized = sanitizePattern.ReplaceAllString(sanitized, "")
	sanitized = whitespacePattern.ReplaceAllString(sanitized, " ")
	return sanitized
}

func fallbackQuestion(question string) string {
	if strings.TrimSpace(question) == "" {
		return "unknown"
	}
	return question
}

func shouldNormalizeTeamResults(sqlText string) bool {
	lower := strings.ToLower(sqlText)
	patterns := []string{
		"mi.winner",
		"d.batting_team",
		"d.bowling_team",
		"p.team_name",
		"wins",
	}
	for _, pattern := range patterns {
		if strings.Contains(lower, pattern) {
			return true
		}
	}
	return false
}

func normalizeTeamResults(rows []map[string]interface{}) []map[string]interface{} {
	if len(rows) == 0 {
		return rows
	}

	canonicalMap := leagues.Default().TeamRenames()

	aggregated := map[string]map[string]interface{}{}
	order := []string{}
	for _, row := range rows {
		teamKey := findTeamKey(row)
		name := strings.TrimSpace(fmt.Sprint(row[teamKey]))
		if name == "" {
			continue
		}
		if canonical, ok := canonicalMap[name]; ok {
			name = canonical
		}

		current, exists := aggregated[name]
		if !exists {
			current = map[string]interface{}{teamKey: name}
			aggregated[name] = current
			order = append(order, name)
		}

		for key, value := range row {
			if key == teamKey {
				continue
			}
			if number, ok := numericValue(value); ok {
				if previous, ok := numericValue(current[key]); ok {
					current[key] = previous + number
				} else {
					current[key] = number
				}
			} else {
				current[key] = value
			}
		}
	}

	result := make([]map[string]interface{}, 0, len(order))
	for _, key := range order {
		result = append(result, aggregated[key])
	}
	return result
}

func findTeamKey(row map[string]interface{}) string {
	for key := range row {
		lower := strings.ToLower(key)
		if lower == "team" || lower == "winner" || lower == "batting_team" || lower == "bowling_team" {
			return key
		}
	}
	for key := range row {
		return key
	}
	return "team"
}

func numericValue(value interface{}) (float64, bool) {
	switch typed := value.(type) {
	case int:
		return float64(typed), true
	case int64:
		return float64(typed), true
	case int32:
		return float64(typed), true
	case float64:
		return typed, true
	case float32:
		return float64(typed), true
	default:
		return 0, false
	}
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}

var (
	yearPattern      = regexp.MustCompile(`\b(20\d\d)\b`)
	limitPattern     = regexp.MustCompile(`(?i)\btop\s+(\d+)\b`)
	wordRegexp       = regexp.MustCompile(`[a-zA-Z]+`)
	cricketStopwords = map[string]bool{
		"runs": true, "wickets": true, "wicket": true, "economy": true, "rate": true, "rates": true,
		"sixes": true, "six": true, "boundaries": true, "boundary": true, "fours": true, "four": true,
		"average": true, "stats": true, "stat": true, "in": true, "ipl": true,
		"wpl": true, "bbl": true, "wbbl": true, "sa20": true, "most": true,
		"top": true, "best": true, "highest": true, "lowest": true, "strike": true, "strikes": true,
		"powerplay": true, "death": true, "overs": true, "over": true, "for": true,
		"the": true, "of": true, "and": true, "total": true, "score": true, "scores": true,
		"scored": true, "taken": true, "what": true, "is": true, "who": true,
		"has": true, "how": true, "many": true, "record": true, "records": true,
		"all": true, "time": true, "career": true, "match": true, "matches": true,
		"season": true, "seasons": true, "by": true, "at": true, "team": true, "teams": true,
		"wins": true, "won": true, "which": true, "vs": true, "against": true,
		"v": true, "matchup": true, "comparison": true, "compare": true,
		"player": true, "players": true, "batsman": true, "batsmen": true, "batter": true, "batters": true,
		"bowler": true, "bowlers": true, "ball": true, "balls": true, "leaderboard": true, "list": true,
		"overall": true, "cap": true, "orange": true, "purple": true,
		"batting": true, "bowling": true, "performance": true, "profile": true, "figures": true,
		"numbers": true, "show": true, "me": true, "give": true, "tell": true, "about": true,
		"does": true, "did": true, "get": true,
		// Pronouns, filler and league words: anything left over is read as a
		// player's name ("his career" once resolved to "Abhishek").
		"his": true, "her": true, "their": true, "he": true, "she": true, "they": true,
		"entire": true, "whole": true, "ever": true, "so": true, "far": true, "since": true,
		"across": true, "league": true, "leagues": true, "t": true, "franchise": true, "domestic": true,
		"women": true, "womens": true, "men": true, "mens": true, "s": true, "competition": true,
		"history": true, "hit": true, "hits": true, "made": true, "have": true, "had": true,
		"with": true, "from": true, "a": true, "an": true, "on": true, "this": true, "year": true,
	}
)

func extractLimit(question string, defaultLimit int) int {
	if match := limitPattern.FindStringSubmatch(question); len(match) > 1 {
		if n, err := strconv.Atoi(match[1]); err == nil && n > 0 && n <= 50 {
			return n
		}
	}
	return defaultLimit
}

func extractPlayerName(question string) (string, string) {
	cleaned := yearPattern.ReplaceAllString(question, "")
	words := wordRegexp.FindAllString(cleaned, -1)
	filtered := make([]string, 0, len(words))
	for _, w := range words {
		if !cricketStopwords[strings.ToLower(w)] {
			filtered = append(filtered, w)
		}
	}
	if len(filtered) == 0 {
		return "", ""
	}
	return strings.Join(filtered, " "), filtered[len(filtered)-1]
}

func extractHeadToHeadPlayers(question string) (string, string) {
	lower := strings.ToLower(question)
	var p1, p2 string
	for _, sep := range []string{" vs ", " against ", " v "} {
		// Split on the lowercased index so "Kohli VS Bumrah" doesn't index past a one-element slice.
		if idx := strings.Index(lower, sep); idx >= 0 {
			p1, p2 = question[:idx], question[idx+len(sep):]
			break
		}
	}

	if p1 != "" && p2 != "" {
		_, s1 := extractPlayerName(p1)
		_, s2 := extractPlayerName(p2)
		return s1, s2
	}
	return "", ""
}

// phaseExpression classifies a delivery's phase for both T20 overs and The
// Hundred's 5-ball sets (powerplay = 6 overs or 5 sets, death = last 5); it
// needs wpl_match_info joined as mi.
const phaseExpression = `(CASE WHEN CAST(SPLIT_PART(d.ball, '.', 1) AS INTEGER) < (CASE WHEN COALESCE(mi.balls_per_over, 6) = 5 THEN 5 ELSE 6 END) THEN 'powerplay' WHEN CAST(SPLIT_PART(d.ball, '.', 1) AS INTEGER) >= 15 THEN 'death' ELSE 'middle' END)`

func buildPhaseFilter(phase string) string {
	switch p := strings.ToLower(strings.TrimSpace(phase)); p {
	case "powerplay", "middle", "death":
		return fmt.Sprintf("%s = '%s'", phaseExpression, p)
	default:
		return ""
	}
}

// legalBallsSQL counts legal deliveries: wides and no-balls are re-bowled.
const legalBallsSQL = "COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)"

var explicitSeasonPattern = regexp.MustCompile(`\b(20\d\d)\s*[-/]\s*(\d\d)\b`)

// buildDateFilter turns a year in the question into a match filter:
// "2023-24" is that season label; "<league> 2024" is the season mostly
// played in 2024 when the league's seasons span two years; any other year is
// the calendar year.
func buildDateFilter(question string, leagueCodes []string, seasons aisql.SeasonIndex) string {
	if m := explicitSeasonPattern.FindStringSubmatch(question); m != nil {
		return fmt.Sprintf("m.season = '%s/%s'", m[1], m[2])
	}
	match := yearPattern.FindString(question)
	if match == "" {
		return ""
	}
	if len(leagueCodes) == 1 {
		year, _ := strconv.Atoi(match)
		if label, ok := seasons.LabelFor(leagueCodes[0], year); ok {
			return fmt.Sprintf("m.season = '%s'", label)
		}
	}
	return fmt.Sprintf("m.start_date >= '%s-01-01' AND m.start_date <= '%s-12-31'", match, match)
}

// playerLookupSQL resolves a surname to the stored player name, preferring
// the given initial and then the player with the most appearances (in the
// question's leagues, when it names any).
func playerLookupSQL(surname, initial, leagueFilter string) string {
	where := fmt.Sprintf("player_name ILIKE '%%%s%%'", surname)
	if leagueFilter != "" {
		where += " AND " + leagueFilter
	}
	// Must start "SELECT player_name FROM wpl_player": that is how the
	// executor recognises a lookup (aisql.HasPlayerNameResolution).
	return fmt.Sprintf(`SELECT player_name FROM wpl_player p JOIN wpl_match m ON m.match_id = p.match_id WHERE %s GROUP BY player_name ORDER BY CASE WHEN player_name ILIKE '%s%%%s' THEN 1 ELSE 2 END, COUNT(*) DESC LIMIT 1`, where, initial, surname)
}

// buildFastPathQueries returns template SQL for a classified question, or nil
// to let Gemini write it. Leagues come from the names in the question (for
// team questions, from the classifier when none is named); with none, a
// question about a specific player covers all leagues and anything else
// covers the men's (or, if asked, women's) leagues.
func buildFastPathQueries(intent, classifiedLeague, phase, question string, seasons aisql.SeasonIndex) []string {
	registry := leagues.Default()
	lowerQ := strings.ToLower(question)

	leagueCodes := aisql.DetectLeagues(registry, question)
	// A team question rarely names its league ("Perth Scorchers wins"); the
	// classifier infers it from the team. Player questions don't use this: a
	// player who appears in several leagues would be wrongly narrowed.
	isTeamQuestion := intent == "team_record" || intent == "team_wins"
	if len(leagueCodes) == 0 && isTeamQuestion && registry.IsVisible(classifiedLeague) {
		leagueCodes = []string{classifiedLeague}
	}
	aboutNamedPlayer := intent == "player_batting_stat" || intent == "player_bowling_stat" || intent == "head_to_head"
	if len(leagueCodes) == 0 && !aboutNamedPlayer {
		leagueCodes = aisql.DefaultLeagues(registry, question)
	}
	leagueFilter := ""
	if len(leagueCodes) > 0 {
		leagueFilter = aisql.LeagueFilterSQL(leagueCodes)
	}
	// Only a league the question named narrows the player lookup.
	lookupLeagueFilter := ""
	if named := aisql.DetectLeagues(registry, question); len(named) > 0 {
		lookupLeagueFilter = aisql.LeagueFilterSQL(named)
	}

	dateFilter := buildDateFilter(question, leagueCodes, seasons)
	phaseFilter := buildPhaseFilter(phase)

	deliveriesFrom := "FROM wpl_delivery d JOIN wpl_match m ON m.match_id = d.match_id"
	if phaseFilter != "" {
		deliveriesFrom += " LEFT JOIN wpl_match_info mi ON mi.match_id = d.match_id"
	}

	baseWhere := func(extra ...string) string {
		parts := []string{"d.innings <= 2"}
		if leagueFilter != "" {
			parts = append(parts, leagueFilter)
		}
		if dateFilter != "" {
			parts = append(parts, dateFilter)
		}
		if phaseFilter != "" {
			parts = append(parts, phaseFilter)
		}
		for _, e := range extra {
			if e != "" {
				parts = append(parts, e)
			}
		}
		return strings.Join(parts, " AND ")
	}

	nameText := aisql.StripLeagueNames(registry, question)
	fullName, surname := extractPlayerName(nameText)
	h2hBatter, h2hBowler := extractHeadToHeadPlayers(nameText)

	if intent == "head_to_head" || (h2hBatter != "" && h2hBowler != "") {
		if h2hBatter == "" || h2hBowler == "" {
			return nil
		}
		q1 := playerLookupSQL(h2hBatter, "", lookupLeagueFilter)
		q2 := playerLookupSQL(h2hBowler, "", lookupLeagueFilter)
		whereClause := baseWhere("d.striker = 'RESOLVED_BATTER_NAME'", "d.bowler = 'RESOLVED_BOWLER_NAME'")
		main := fmt.Sprintf(`SELECT SUM(d.runs_off_bat) AS runs, COUNT(*) FILTER (WHERE d.wides = 0) AS balls, ROUND(((SUM(d.runs_off_bat)::DECIMAL * 100) / NULLIF(COUNT(*) FILTER (WHERE d.wides = 0), 0)), 2) AS strike_rate, COUNT(CASE WHEN d.player_dismissed = d.striker THEN 1 END) AS dismissals %s WHERE %s`, deliveriesFrom, whereClause)
		return []string{q1, q2, main}
	}

	switch intent {
	case "player_batting_stat":
		if surname == "" || len(surname) < 2 {
			return nil
		}
		initial := ""
		if len(fullName) > 0 {
			initial = string(fullName[0])
		}
		lookup := playerLookupSQL(surname, initial, lookupLeagueFilter)
		whereClause := baseWhere("d.striker = 'RESOLVED_PLAYER_NAME'")
		main := fmt.Sprintf(`SELECT d.striker, SUM(d.runs_off_bat) AS runs, COUNT(*) FILTER (WHERE d.wides = 0) AS balls, ROUND(((SUM(d.runs_off_bat)::DECIMAL * 100) / NULLIF(COUNT(*) FILTER (WHERE d.wides = 0), 0)), 2) AS strike_rate, COUNT(*) FILTER (WHERE d.runs_off_bat = 4) AS fours, COUNT(*) FILTER (WHERE d.runs_off_bat = 6) AS sixes %s WHERE %s GROUP BY d.striker`, deliveriesFrom, whereClause)
		return []string{lookup, main}

	case "player_bowling_stat":
		if surname == "" || len(surname) < 2 {
			return nil
		}
		initial := ""
		if len(fullName) > 0 {
			initial = string(fullName[0])
		}
		lookup := playerLookupSQL(surname, initial, lookupLeagueFilter)
		whereClause := baseWhere("d.bowler = 'RESOLVED_PLAYER_NAME'")
		main := fmt.Sprintf(`SELECT d.bowler, COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')) AS wickets, SUM(d.runs_off_bat + d.wides + d.noballs) AS runs_conceded, ROUND(SUM(d.runs_off_bat + d.wides + d.noballs) / NULLIF(`+legalBallsSQL+`::DECIMAL / 6, 0), 2) AS economy_rate %s WHERE %s GROUP BY d.bowler`, deliveriesFrom, whereClause)
		return []string{lookup, main}

	case "most_sixes":
		if strings.Contains(lowerQ, "team") {
			return nil
		}
		limit := extractLimit(question, 10)
		whereClause := baseWhere()
		return []string{fmt.Sprintf(`SELECT d.striker, COUNT(*) FILTER (WHERE d.runs_off_bat = 6) AS sixes, SUM(d.runs_off_bat) AS runs, ROUND(((SUM(d.runs_off_bat)::DECIMAL * 100) / NULLIF(COUNT(*) FILTER (WHERE d.wides = 0), 0)), 2) AS strike_rate %s WHERE %s GROUP BY d.striker ORDER BY sixes DESC LIMIT %d`, deliveriesFrom, whereClause, limit)}

	case "team_record", "team_wins":
		limit := extractLimit(question, 10)
		whereParts := []string{"mi.winner IS NOT NULL"}
		if leagueFilter != "" {
			whereParts = append(whereParts, leagueFilter)
		}
		if dateFilter != "" {
			whereParts = append(whereParts, dateFilter)
		}
		return []string{fmt.Sprintf(`SELECT COALESCE(mi.winner, 'Unknown') AS team, COUNT(*) AS total_wins FROM wpl_match_info mi JOIN wpl_match m ON m.match_id = mi.match_id WHERE %s GROUP BY mi.winner ORDER BY total_wins DESC LIMIT %d`, strings.Join(whereParts, " AND "), limit)}

	case "leading_wicket_takers":
		limit := extractLimit(question, 10)
		whereClause := baseWhere()
		return []string{fmt.Sprintf(`SELECT d.bowler, COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')) AS wickets, ROUND(SUM(d.runs_off_bat + d.wides + d.noballs) / NULLIF(`+legalBallsSQL+`::DECIMAL / 6, 0), 2) AS economy_rate %s WHERE %s GROUP BY d.bowler ORDER BY wickets DESC LIMIT %d`, deliveriesFrom, whereClause, limit)}

	case "leading_run_scorers":
		limit := extractLimit(question, 10)
		whereClause := baseWhere()
		return []string{fmt.Sprintf(`SELECT d.striker, SUM(d.runs_off_bat) AS runs, COUNT(*) FILTER (WHERE d.wides = 0) AS balls, ROUND(((SUM(d.runs_off_bat)::DECIMAL * 100) / NULLIF(COUNT(*) FILTER (WHERE d.wides = 0), 0)), 2) AS strike_rate %s WHERE %s GROUP BY d.striker ORDER BY runs DESC LIMIT %d`, deliveriesFrom, whereClause, limit)}

	default:
		return nil
	}
}
