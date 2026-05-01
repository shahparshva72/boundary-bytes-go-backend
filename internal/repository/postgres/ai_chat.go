package postgres

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func (s *service) LogAIRequest(ctx context.Context, params models.LogAIRequestParams) (string, error) {
	id := newAIRequestID()
	query := `
		INSERT INTO ai_chat_request (
			id,
			question,
			sanitized_question,
			league,
			generated_sql,
			row_count,
			execution_time_ms,
			success,
			error_code,
			error_message
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	`

	_, err := s.db.ExecContext(
		ctx,
		query,
		id,
		params.Question,
		params.SanitizedQuestion,
		params.League,
		params.GeneratedSQL,
		params.RowCount,
		params.ExecutionTimeMS,
		params.Success,
		params.ErrorCode,
		params.ErrorMessage,
	)
	if err != nil {
		return "", err
	}

	return id, nil
}

func (s *service) GetAIRequestByID(ctx context.Context, id string) (*models.AIChatRequestRecord, error) {
	query := `
		SELECT
			id,
			question,
			sanitized_question,
			league,
			generated_sql,
			row_count,
			execution_time_ms,
			success,
			error_code,
			error_message,
			is_accurate,
			feedback_note,
			feedback_at,
			created_at
		FROM ai_chat_request
		WHERE id = $1
	`

	var record models.AIChatRequestRecord
	var sanitizedQuestion sql.NullString
	var league sql.NullString
	var generatedSQL sql.NullString
	var rowCount sql.NullInt64
	var executionTimeMS sql.NullInt64
	var errorCode sql.NullString
	var errorMessage sql.NullString
	var isAccurate sql.NullBool
	var feedbackNote sql.NullString
	var feedbackAt sql.NullTime

	err := s.db.QueryRowContext(ctx, query, id).Scan(
		&record.ID,
		&record.Question,
		&sanitizedQuestion,
		&league,
		&generatedSQL,
		&rowCount,
		&executionTimeMS,
		&record.Success,
		&errorCode,
		&errorMessage,
		&isAccurate,
		&feedbackNote,
		&feedbackAt,
		&record.CreatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	record.SanitizedQuestion = nullableStringPtr(sanitizedQuestion)
	record.League = nullableStringPtr(league)
	record.GeneratedSQL = nullableStringPtr(generatedSQL)
	record.RowCount = nullableIntPtr(rowCount)
	record.ExecutionTimeMS = nullableIntPtr(executionTimeMS)
	record.ErrorCode = nullableStringPtr(errorCode)
	record.ErrorMessage = nullableStringPtr(errorMessage)
	if isAccurate.Valid {
		record.IsAccurate = &isAccurate.Bool
	}
	record.FeedbackNote = nullableStringPtr(feedbackNote)
	if feedbackAt.Valid {
		record.FeedbackAt = &feedbackAt.Time
	}

	return &record, nil
}

func (s *service) MarkAIRequestAccuracy(ctx context.Context, requestID string, isAccurate bool, feedbackNote *string) error {
	query := `
		UPDATE ai_chat_request
		SET is_accurate = $2,
			feedback_note = $3,
			feedback_at = CURRENT_TIMESTAMP
		WHERE id = $1
	`
	result, err := s.db.ExecContext(ctx, query, requestID, isAccurate, feedbackNote)
	if err != nil {
		return err
	}

	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}

	return nil
}

func (s *service) GetAIAccuracyStats(ctx context.Context) (models.AIFeedbackStats, error) {
	query := `
		SELECT
			COUNT(*) FILTER (WHERE is_accurate IS NOT NULL)::int AS total,
			COUNT(*) FILTER (WHERE is_accurate IS TRUE)::int AS accurate,
			COUNT(*) FILTER (WHERE is_accurate IS FALSE)::int AS inaccurate
		FROM ai_chat_request
	`

	var stats models.AIFeedbackStats
	if err := s.db.QueryRowContext(ctx, query).Scan(&stats.Total, &stats.Accurate, &stats.Inaccurate); err != nil {
		return models.AIFeedbackStats{}, err
	}
	if stats.Total > 0 {
		stats.AccuracyRate = (float64(stats.Accurate) / float64(stats.Total)) * 100
	}

	return stats, nil
}

func (s *service) ExecuteAIQuery(ctx context.Context, query string) (models.AIQueryResult, error) {
	start := time.Now()
	cleanQuery := stringsTrimTrailingSemicolon(query)

	rows, err := s.db.QueryContext(ctx, cleanQuery)
	if err != nil {
		return models.AIQueryResult{}, err
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return models.AIQueryResult{}, err
	}

	data := []map[string]interface{}{}
	for rows.Next() {
		values := make([]interface{}, len(columnNames))
		valuePtrs := make([]interface{}, len(columnNames))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return models.AIQueryResult{}, err
		}

		row := make(map[string]interface{}, len(columnNames))
		for i, name := range columnNames {
			row[name] = normalizeAIQueryValue(values[i])
		}
		data = append(data, row)
	}

	if err := rows.Err(); err != nil {
		return models.AIQueryResult{}, err
	}

	return models.AIQueryResult{
		Data:            data,
		RowCount:        len(data),
		ExecutionTimeMS: int(time.Since(start).Milliseconds()),
	}, nil
}

func newAIRequestID() string {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("go_%d", time.Now().UnixNano())
	}
	return "go_" + hex.EncodeToString(buffer)
}

func nullableStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}

func nullableIntPtr(value sql.NullInt64) *int {
	if !value.Valid {
		return nil
	}
	converted := int(value.Int64)
	return &converted
}

func normalizeAIQueryValue(value interface{}) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return normalizeAIQueryString(string(typed))
	case string:
		return typed
	case time.Time:
		return typed.Format(time.RFC3339)
	case int64:
		return typed
	case int32:
		return typed
	case int:
		return typed
	case float64:
		return typed
	case float32:
		return typed
	case bool:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func normalizeAIQueryString(value string) interface{} {
	if parsed, err := strconv.ParseInt(value, 10, 64); err == nil {
		return parsed
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return parsed
	}
	return value
}

func stringsTrimTrailingSemicolon(value string) string {
	for {
		trimmed := strings.TrimSpace(value)
		next := strings.TrimSuffix(trimmed, ";")
		if next == trimmed {
			return trimmed
		}
		value = next
	}
}
