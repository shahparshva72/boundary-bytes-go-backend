package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	aisql "github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/service/texttosql"
)

type textToSQLRequest struct {
	Question string `json:"question"`
}

func TextToSQL(service *texttosql.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body textToSQLRequest

		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			service.LogInvalidRequest(r.Context(), "Invalid request format")
			writeJSON(w, http.StatusBadRequest, formatAIError("Invalid request format", "VALIDATION_ERROR"))
			return
		}

		result, err := service.Answer(r.Context(), body.Question)
		if err != nil {
			status, payload := classifyTextToSQLError(err)
			writeJSON(w, status, payload)
			return
		}

		writeJSON(w, http.StatusOK, textToSQLSuccessResponse{
			Success: true,
			Data:    result.Data,
			Metadata: textToSQLMetadata{
				RowCount:      result.RowCount,
				ExecutionTime: result.ExecutionTimeMS,
				GeneratedSQL:  result.GeneratedSQL,
			},
			RequestID: result.RequestID,
		})
	}
}

func classifyTextToSQLError(err error) (int, apiErrorResponse) {
	var serviceError *texttosql.Error
	if !errors.As(err, &serviceError) {
		return http.StatusInternalServerError, formatAIServerError()
	}

	switch serviceError.Code {
	case texttosql.CodeValidation:
		return http.StatusBadRequest, formatAIError(serviceError.Message, serviceError.Code)
	case texttosql.CodeSQL:
		return http.StatusBadRequest, formatAIError(serviceError.Message, serviceError.Code)
	case texttosql.CodeDatabase:
		return classifyDatabaseError(serviceError.Message, serviceError.SanitizedQuestion)
	default:
		return classifyAIGenerationError(serviceError.Message, serviceError.SanitizedQuestion)
	}
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
