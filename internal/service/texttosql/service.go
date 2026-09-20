package texttosql

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"strconv"
	"strings"
	"time"

	aisql "github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
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
	lookupTimeout  time.Duration
	executeTimeout time.Duration
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
	}
}

func (s *Service) WithClassifier(classifier QueryClassifier) *Service {
	s.classifier = classifier
	return s
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

			// Domain Guardrail: reject non-cricket questions in ~80ms without hitting Gemini
			if !classification.IsCricket && classification.Confidence >= 0.80 {
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
				if fastQueries := buildFastPathQueries(classification.Intent, classification.League, classification.Phase, sanitizedQuestion); len(fastQueries) > 0 {
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

		result, err := s.repository.ExecuteAIQuery(queryCtx, sqlText)
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

		queryResult, err := s.repository.ExecuteAIQuery(queryCtx, finalSQL)
		if err != nil {
			return "", nil, 0, err
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

	canonicalMap := map[string]string{
		"Royal Challengers Bengaluru": "Royal Challengers Bangalore",
		"Delhi Daredevils":            "Delhi Capitals",
		"Kings XI Punjab":             "Punjab Kings",
		"Rising Pune Supergiants":     "Rising Pune Supergiant",
	}

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
	if strings.Contains(lower, " vs ") {
		parts := strings.SplitN(question, " vs ", 2)
		p1, p2 = parts[0], parts[1]
	} else if strings.Contains(lower, " against ") {
		parts := strings.SplitN(question, " against ", 2)
		p1, p2 = parts[0], parts[1]
	} else if strings.Contains(lower, " v ") {
		parts := strings.SplitN(question, " v ", 2)
		p1, p2 = parts[0], parts[1]
	}

	if p1 != "" && p2 != "" {
		_, s1 := extractPlayerName(p1)
		_, s2 := extractPlayerName(p2)
		return s1, s2
	}
	return "", ""
}

func buildPhaseFilter(phase string) string {
	switch strings.ToLower(strings.TrimSpace(phase)) {
	case "powerplay":
		return "CAST(SPLIT_PART(d.ball, '.', 1) AS INTEGER) BETWEEN 0 AND 5"
	case "middle":
		return "CAST(SPLIT_PART(d.ball, '.', 1) AS INTEGER) BETWEEN 6 AND 14"
	case "death":
		return "CAST(SPLIT_PART(d.ball, '.', 1) AS INTEGER) BETWEEN 15 AND 19"
	default:
		return ""
	}
}

func buildFastPathQueries(intent, league, phase, question string) []string {
	var leagueFilter string
	lowerQ := strings.ToLower(question)
	normalizedLeague := strings.ToUpper(strings.TrimSpace(league))
	switch {
	case strings.Contains(lowerQ, "wpl") || normalizedLeague == "WPL":
		leagueFilter = "m.league = 'WPL'"
	case strings.Contains(lowerQ, "bbl") || normalizedLeague == "BBL":
		leagueFilter = "m.league = 'BBL'"
	case strings.Contains(lowerQ, "wbbl") || normalizedLeague == "WBBL":
		leagueFilter = "m.league = 'WBBL'"
	case strings.Contains(lowerQ, "sa20") || normalizedLeague == "SA20":
		leagueFilter = "m.league = 'SA20'"
	case strings.Contains(lowerQ, "ipl") || normalizedLeague == "IPL":
		leagueFilter = "m.league = 'IPL'"
	default:
		leagueFilter = "m.league = 'IPL'"
	}

	var dateFilter string
	if match := yearPattern.FindString(question); match != "" {
		dateFilter = fmt.Sprintf("m.start_date >= '%s-01-01' AND m.start_date <= '%s-12-31'", match, match)
	}

	phaseFilter := buildPhaseFilter(phase)

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

	fullName, surname := extractPlayerName(question)
	h2hBatter, h2hBowler := extractHeadToHeadPlayers(question)

	if intent == "head_to_head" || (h2hBatter != "" && h2hBowler != "") {
		if h2hBatter == "" || h2hBowler == "" {
			return nil
		}
		q1 := fmt.Sprintf(`SELECT player_name FROM wpl_player WHERE player_name ILIKE '%%%s%%' ORDER BY CASE WHEN player_name ILIKE '%s%%' THEN 1 ELSE 2 END LIMIT 1`, h2hBatter, h2hBatter)
		q2 := fmt.Sprintf(`SELECT player_name FROM wpl_player WHERE player_name ILIKE '%%%s%%' ORDER BY CASE WHEN player_name ILIKE '%s%%' THEN 1 ELSE 2 END LIMIT 1`, h2hBowler, h2hBowler)
		whereClause := baseWhere("d.striker = 'RESOLVED_BATTER_NAME'", "d.bowler = 'RESOLVED_BOWLER_NAME'")
		main := fmt.Sprintf(`SELECT SUM(d.runs_off_bat) AS runs, COUNT(*) FILTER (WHERE d.wides = 0) AS balls, ROUND(((SUM(d.runs_off_bat)::DECIMAL * 100) / NULLIF(COUNT(*) FILTER (WHERE d.wides = 0), 0)), 2) AS strike_rate, COUNT(CASE WHEN d.player_dismissed = d.striker THEN 1 END) AS dismissals FROM wpl_delivery d JOIN wpl_match m ON m.match_id = d.match_id WHERE %s`, whereClause)
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
		lookup := fmt.Sprintf(`SELECT player_name FROM wpl_player WHERE player_name ILIKE '%%%s%%' ORDER BY CASE WHEN player_name ILIKE '%s%%%s' THEN 1 ELSE 2 END LIMIT 1`, surname, initial, surname)
		whereClause := baseWhere("d.striker = 'RESOLVED_PLAYER_NAME'")
		main := fmt.Sprintf(`SELECT d.striker, SUM(d.runs_off_bat) AS runs, COUNT(*) FILTER (WHERE d.wides = 0) AS balls, ROUND(((SUM(d.runs_off_bat)::DECIMAL * 100) / NULLIF(COUNT(*) FILTER (WHERE d.wides = 0), 0)), 2) AS strike_rate, COUNT(*) FILTER (WHERE d.runs_off_bat = 4) AS fours, COUNT(*) FILTER (WHERE d.runs_off_bat = 6) AS sixes FROM wpl_delivery d JOIN wpl_match m ON m.match_id = d.match_id WHERE %s GROUP BY d.striker`, whereClause)
		return []string{lookup, main}

	case "player_bowling_stat":
		if surname == "" || len(surname) < 2 {
			return nil
		}
		initial := ""
		if len(fullName) > 0 {
			initial = string(fullName[0])
		}
		lookup := fmt.Sprintf(`SELECT player_name FROM wpl_player WHERE player_name ILIKE '%%%s%%' ORDER BY CASE WHEN player_name ILIKE '%s%%%s' THEN 1 ELSE 2 END LIMIT 1`, surname, initial, surname)
		whereClause := baseWhere("d.bowler = 'RESOLVED_PLAYER_NAME'")
		main := fmt.Sprintf(`SELECT d.bowler, COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')) AS wickets, SUM(d.runs_off_bat + d.wides + d.noballs) AS runs_conceded, ROUND(SUM(d.runs_off_bat + d.wides + d.noballs) / NULLIF(COUNT(*)::DECIMAL / 6, 0), 2) AS economy_rate FROM wpl_delivery d JOIN wpl_match m ON m.match_id = d.match_id WHERE %s GROUP BY d.bowler`, whereClause)
		return []string{lookup, main}

	case "most_sixes":
		if strings.Contains(lowerQ, "team") {
			return nil
		}
		limit := extractLimit(question, 10)
		whereClause := baseWhere()
		return []string{fmt.Sprintf(`SELECT d.striker, COUNT(*) FILTER (WHERE d.runs_off_bat = 6) AS sixes, SUM(d.runs_off_bat) AS runs, ROUND(((SUM(d.runs_off_bat)::DECIMAL * 100) / NULLIF(COUNT(*) FILTER (WHERE d.wides = 0), 0)), 2) AS strike_rate FROM wpl_delivery d JOIN wpl_match m ON m.match_id = d.match_id WHERE %s GROUP BY d.striker ORDER BY sixes DESC LIMIT %d`, whereClause, limit)}

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
		return []string{fmt.Sprintf(`SELECT d.bowler, COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')) AS wickets, ROUND(SUM(d.runs_off_bat + d.wides + d.noballs) / NULLIF(COUNT(*)::DECIMAL / 6, 0), 2) AS economy_rate FROM wpl_delivery d JOIN wpl_match m ON m.match_id = d.match_id WHERE %s GROUP BY d.bowler ORDER BY wickets DESC LIMIT %d`, whereClause, limit)}

	case "leading_run_scorers":
		limit := extractLimit(question, 10)
		whereClause := baseWhere()
		return []string{fmt.Sprintf(`SELECT d.striker, SUM(d.runs_off_bat) AS runs, COUNT(*) FILTER (WHERE d.wides = 0) AS balls, ROUND(((SUM(d.runs_off_bat)::DECIMAL * 100) / NULLIF(COUNT(*) FILTER (WHERE d.wides = 0), 0)), 2) AS strike_rate FROM wpl_delivery d JOIN wpl_match m ON m.match_id = d.match_id WHERE %s GROUP BY d.striker ORDER BY runs DESC LIMIT %d`, whereClause, limit)}

	default:
		return nil
	}
}
