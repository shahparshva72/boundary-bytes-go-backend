package handlers

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
)

type feedbackRequest struct {
	RequestID    string  `json:"requestId"`
	IsAccurate   *bool   `json:"isAccurate"`
	FeedbackNote *string `json:"feedbackNote,omitempty"`
}

func SubmitAIFeedback(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var request feedbackRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeJSON(w, http.StatusBadRequest, formatAIError("Invalid request format", "VALIDATION_ERROR"))
			return
		}

		request.RequestID = strings.TrimSpace(request.RequestID)
		if request.RequestID == "" {
			writeJSON(w, http.StatusBadRequest, formatAIError("Request ID is required", "VALIDATION_ERROR"))
			return
		}
		if request.IsAccurate == nil {
			writeJSON(w, http.StatusBadRequest, formatAIError("isAccurate is required", "VALIDATION_ERROR"))
			return
		}
		if request.FeedbackNote != nil {
			note := strings.TrimSpace(*request.FeedbackNote)
			if len(note) > 1000 {
				writeJSON(w, http.StatusBadRequest, formatAIError("Feedback note is too long (max 1000 characters)", "VALIDATION_ERROR"))
				return
			}
			if note == "" {
				request.FeedbackNote = nil
			} else {
				request.FeedbackNote = &note
			}
		}

		existingRequest, err := db.GetAIRequestByID(r.Context(), request.RequestID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, formatAIError("Failed to submit feedback. Please try again.", "SERVER_ERROR"))
			return
		}
		if existingRequest == nil {
			writeJSON(w, http.StatusNotFound, formatAIError("Request not found", "NOT_FOUND"))
			return
		}
		if existingRequest.IsAccurate != nil {
			writeJSON(w, http.StatusConflict, formatAIError("Feedback has already been submitted for this request", "CONFLICT"))
			return
		}

		if err := db.MarkAIRequestAccuracy(r.Context(), request.RequestID, *request.IsAccurate, request.FeedbackNote); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				writeJSON(w, http.StatusNotFound, formatAIError("Request not found", "NOT_FOUND"))
				return
			}
			writeJSON(w, http.StatusInternalServerError, formatAIError("Failed to submit feedback. Please try again.", "SERVER_ERROR"))
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"message": "Feedback submitted successfully",
		})
	}
}

func GetAIFeedbackStats(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		stats, err := db.GetAIAccuracyStats(r.Context())
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
