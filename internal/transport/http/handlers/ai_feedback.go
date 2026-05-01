package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/service/aifeedback"
)

type feedbackRequest struct {
	RequestID    string  `json:"requestId"`
	IsAccurate   *bool   `json:"isAccurate"`
	FeedbackNote *string `json:"feedbackNote,omitempty"`
}

func SubmitAIFeedback(service *aifeedback.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request feedbackRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, formatAIError("Invalid request format", "VALIDATION_ERROR"))
			return
		}

		err := service.Submit(r.Context(), aifeedback.SubmitParams{
			RequestID:    request.RequestID,
			IsAccurate:   request.IsAccurate,
			FeedbackNote: request.FeedbackNote,
		})
		if err != nil {
			writeAIFeedbackError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Feedback submitted successfully",
		})
	}
}

func GetAIFeedbackStats(service *aifeedback.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := service.Stats(r.Context())
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, formatAIError("Failed to fetch statistics", "SERVER_ERROR"))
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    stats,
		})
	}
}

func writeAIFeedbackError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, aifeedback.ErrRequestIDRequired),
		errors.Is(err, aifeedback.ErrAccuracyRequired),
		errors.Is(err, aifeedback.ErrFeedbackNoteTooLong):
		writeJSON(w, http.StatusBadRequest, formatAIError(err.Error(), "VALIDATION_ERROR"))
	case errors.Is(err, aifeedback.ErrRequestNotFound):
		writeJSON(w, http.StatusNotFound, formatAIError(err.Error(), "NOT_FOUND"))
	case errors.Is(err, aifeedback.ErrFeedbackAlreadyProvided):
		writeJSON(w, http.StatusConflict, formatAIError(err.Error(), "CONFLICT"))
	default:
		writeJSON(w, http.StatusInternalServerError, formatAIError("Failed to submit feedback. Please try again.", "SERVER_ERROR"))
	}
}
