package texttosql

import (
	"context"
	"fmt"
	"regexp"
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

type Repository interface {
	ExecuteAIQuery(ctx context.Context, query string) (models.AIQueryResult, error)
	LogAIRequest(ctx context.Context, params models.LogAIRequestParams) (string, error)
}

type Service struct {
	repository     Repository
	generator      SQLGenerator
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
	rawGeneratedQueries, err := s.generator.GenerateSQL(ctx, sanitizedQuestion)
	if err != nil {
		return nil, s.recordError(ctx, CodeAI, question, sanitizedQuestion, "", err)
	}

	if len(rawGeneratedQueries) > 1 {
		if err := aisql.ValidateSequentialQueries(rawGeneratedQueries); err != nil {
			return nil, &Error{Code: CodeSQL, Message: err.Error(), SanitizedQuestion: sanitizedQuestion}
		}
	}

	runQuery := func(sqlText string) ([]map[string]interface{}, error) {
		validation := aisql.ValidateSQL(sqlText)
		if !validation.IsValid {
			return nil, fmt.Errorf("Generated lookup query failed security validation: %s", strings.Join(validation.Errors, ", "))
		}

		queryCtx, cancel := context.WithTimeout(ctx, s.lookupTimeout)
		defer cancel()

		result, err := s.repository.ExecuteAIQuery(queryCtx, sqlText)
		if err != nil {
			return nil, err
		}
		return result.Data, nil
	}

	builtQueries, err := aisql.BuildExecutableQueries(rawGeneratedQueries, runQuery)
	if err != nil {
		return nil, s.recordError(ctx, CodeAI, question, sanitizedQuestion, "", err)
	}

	finalSQL := builtQueries[len(builtQueries)-1]
	validation := aisql.ValidateSQL(finalSQL)
	if !validation.IsValid {
		message := "Generated query failed security validation: " + strings.Join(validation.Errors, ", ")
		return nil, &Error{Code: CodeSQL, Message: message, SanitizedQuestion: sanitizedQuestion}
	}

	queryCtx, cancel := context.WithTimeout(ctx, s.executeTimeout)
	queryResult, err := s.repository.ExecuteAIQuery(queryCtx, finalSQL)
	cancel()
	if err != nil {
		return nil, s.recordError(ctx, CodeDatabase, question, sanitizedQuestion, finalSQL, err)
	}

	formattedData := queryResult.Data
	if shouldNormalizeTeamResults(finalSQL) {
		formattedData = normalizeTeamResults(formattedData)
	}

	requestID := s.log(ctx, models.LogAIRequestParams{
		Question:          question,
		SanitizedQuestion: &sanitizedQuestion,
		GeneratedSQL:      &finalSQL,
		RowCount:          intPtr(len(formattedData)),
		ExecutionTimeMS:   &queryResult.ExecutionTimeMS,
		Success:           true,
	})

	return &Result{
		Data:              formattedData,
		RowCount:          len(formattedData),
		ExecutionTimeMS:   int(time.Since(start).Milliseconds()),
		GeneratedSQL:      finalSQL,
		RequestID:         requestID,
		SanitizedQuestion: sanitizedQuestion,
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
