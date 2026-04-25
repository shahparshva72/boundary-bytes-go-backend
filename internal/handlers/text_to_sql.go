package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	aisql "github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

var validTextToSQLQuestionPattern = regexp.MustCompile(`^[a-zA-Z0-9\s?.,\-'"()\/:%+&]+$`)
var sanitizeTextToSQLPattern = regexp.MustCompile(`[^\w\s?.,\-'"()\/:%+&]`)
var whitespacePattern = regexp.MustCompile(`\s+`)

type textToSQLRequest struct {
	Question string `json:"question"`
}

func TextToSQL(db database.Service, generator aisql.SQLGenerator) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		var body textToSQLRequest

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			logAIRequest(r.Context(), db, models.LogAIRequestParams{
				Question:     "unknown",
				Success:      false,
				ErrorCode:    stringPtr("VALIDATION_ERROR"),
				ErrorMessage: stringPtr("Invalid request format"),
			})
			writeJSON(w, http.StatusBadRequest, formatAIError("Invalid request format", "VALIDATION_ERROR"))
			return
		}

		if generator == nil {
			writeJSON(w, http.StatusServiceUnavailable, formatAIError("AI service configuration error", "AI_ERROR"))
			return
		}

		if errMessage := validateTextToSQLQuestion(body.Question); errMessage != "" {
			logAIRequest(r.Context(), db, models.LogAIRequestParams{
				Question:     fallbackQuestion(body.Question),
				Success:      false,
				ErrorCode:    stringPtr("VALIDATION_ERROR"),
				ErrorMessage: &errMessage,
			})
			writeJSON(w, http.StatusBadRequest, formatAIError(errMessage, "VALIDATION_ERROR"))
			return
		}

		sanitizedQuestion := sanitizeTextToSQLQuestion(body.Question)
		rawGeneratedQueries := []string{}
		cachedSQL, err := db.GetCachedAIQuery(r.Context(), sanitizedQuestion)
		if err == nil && cachedSQL != nil {
			rawGeneratedQueries = []string{*cachedSQL}
		} else {
			rawGeneratedQueries, err = generator.GenerateSQL(r.Context(), sanitizedQuestion)
			if err != nil {
				errorMessage := err.Error()
				logAIRequest(r.Context(), db, models.LogAIRequestParams{
					Question:          body.Question,
					SanitizedQuestion: &sanitizedQuestion,
					Success:           false,
					ErrorCode:         stringPtr("SQL_GENERATION_ERROR"),
					ErrorMessage:      &errorMessage,
				})

				status, payload := classifyAIGenerationError(errorMessage, sanitizedQuestion)
				writeJSON(w, status, payload)
				return
			}
		}

		if len(rawGeneratedQueries) > 1 {
			if err := aisql.ValidateSequentialQueries(rawGeneratedQueries); err != nil {
				writeJSON(w, http.StatusBadRequest, formatAIError(err.Error(), "SQL_ERROR"))
				return
			}
		}

		runQuery := func(sqlText string) ([]map[string]interface{}, error) {
			validation := aisql.ValidateSQL(sqlText)
			if !validation.IsValid {
				return nil, fmt.Errorf("Generated lookup query failed security validation: %s", strings.Join(validation.Errors, ", "))
			}
			queryCtx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			defer cancel()

			result, err := db.ExecuteAIQuery(queryCtx, sqlText)
			if err != nil {
				return nil, err
			}
			return result.Data, nil
		}

		builtQueries, err := aisql.BuildExecutableQueries(rawGeneratedQueries, runQuery)
		if err != nil {
			errorMessage := err.Error()
			logAIRequest(r.Context(), db, models.LogAIRequestParams{
				Question:          body.Question,
				SanitizedQuestion: &sanitizedQuestion,
				Success:           false,
				ErrorCode:         stringPtr("SQL_GENERATION_ERROR"),
				ErrorMessage:      &errorMessage,
			})
			status, payload := classifyAIGenerationError(errorMessage, sanitizedQuestion)
			writeJSON(w, status, payload)
			return
		}

		finalSQL := builtQueries[len(builtQueries)-1]
		validation := aisql.ValidateSQL(finalSQL)
		if !validation.IsValid {
			message := "Generated query failed security validation: " + strings.Join(validation.Errors, ", ")
			writeJSON(w, http.StatusBadRequest, formatAIError(message, "SQL_ERROR"))
			return
		}

		queryCtx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		queryResult, err := db.ExecuteAIQuery(queryCtx, finalSQL)
		cancel()
		if err != nil {
			errorMessage := err.Error()
			logAIRequest(r.Context(), db, models.LogAIRequestParams{
				Question:          body.Question,
				SanitizedQuestion: &sanitizedQuestion,
				GeneratedSQL:      &finalSQL,
				Success:           false,
				ErrorCode:         stringPtr("DATABASE_ERROR"),
				ErrorMessage:      &errorMessage,
			})
			status, payload := classifyDatabaseError(errorMessage, sanitizedQuestion)
			writeJSON(w, status, payload)
			return
		}

		formattedData := queryResult.Data
		if shouldNormalizeTeamResults(finalSQL) {
			formattedData = normalizeTeamResults(formattedData)
		}

		totalExecutionTime := int(time.Since(start).Milliseconds())
		requestID := logAIRequest(r.Context(), db, models.LogAIRequestParams{
			Question:          body.Question,
			SanitizedQuestion: &sanitizedQuestion,
			GeneratedSQL:      &finalSQL,
			RowCount:          intPtr(len(formattedData)),
			ExecutionTimeMS:   &queryResult.ExecutionTimeMS,
			Success:           true,
		})

		writeJSON(w, http.StatusOK, textToSQLSuccessResponse{
			Success: true,
			Data:    formattedData,
			Metadata: textToSQLMetadata{
				RowCount:      len(formattedData),
				ExecutionTime: totalExecutionTime,
				GeneratedSQL:  finalSQL,
			},
			RequestID: requestID,
		})
	}
}

func validateTextToSQLQuestion(question string) string {
	if question == "" {
		return "Question cannot be empty"
	}
	if len(question) > 500 {
		return "Question is too long"
	}
	if !validTextToSQLQuestionPattern.MatchString(question) {
		return "Question contains invalid characters. Only letters, numbers, spaces, parentheses, and common punctuation are allowed."
	}
	return ""
}

func sanitizeTextToSQLQuestion(question string) string {
	sanitized := strings.TrimSpace(question)
	sanitized = sanitizeTextToSQLPattern.ReplaceAllString(sanitized, "")
	sanitized = whitespacePattern.ReplaceAllString(sanitized, " ")
	return sanitized
}

func fallbackQuestion(question string) string {
	if strings.TrimSpace(question) == "" {
		return "unknown"
	}
	return question
}

func classifyAIGenerationError(message, sanitizedQuestion string) (int, apiErrorResponse) {
	lower := strings.ToLower(message)
	switch {
	case message == aisql.ErrMissingAPIKey.Error():
		return http.StatusServiceUnavailable, formatAIError("AI service configuration error", "AI_ERROR")
	case strings.Contains(lower, "rate limit") || strings.Contains(lower, "quota"):
		return http.StatusTooManyRequests, formatAIError("AI service rate limit exceeded. Please try again in a moment.", "RATE_LIMIT_ERROR")
	case strings.Contains(lower, "unavailable") || strings.Contains(lower, "service"):
		return http.StatusServiceUnavailable, formatAIUnavailableError()
	default:
		return http.StatusInternalServerError, formatAIError(message, "AI_ERROR", sanitizedQuestion)
	}
}

func classifyDatabaseError(message, sanitizedQuestion string) (int, apiErrorResponse) {
	lower := strings.ToLower(message)
	switch {
	case strings.Contains(lower, "timeout") || strings.Contains(lower, "took too long") || strings.Contains(lower, "context deadline exceeded"):
		return http.StatusRequestTimeout, formatAITimeoutError(sanitizedQuestion)
	case strings.Contains(lower, "connection") || strings.Contains(lower, "connect"):
		return http.StatusServiceUnavailable, formatAIError("Database connection issue. Please try again in a moment.", "DATABASE_ERROR")
	default:
		return http.StatusInternalServerError, formatAIError(message, "DATABASE_ERROR", sanitizedQuestion)
	}
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

func logAIRequest(ctx context.Context, db database.Service, params models.LogAIRequestParams) string {
	id, err := db.LogAIRequest(ctx, params)
	if err != nil {
		return fmt.Sprintf("log-failed-%d", time.Now().UnixMilli())
	}
	return id
}

func stringPtr(value string) *string {
	return &value
}

func intPtr(value int) *int {
	return &value
}
