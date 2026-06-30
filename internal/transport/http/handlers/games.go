package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	gamesservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/games"
	statsservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/stats"
)

func GetMatchupRound(service *statsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		seed := r.URL.Query().Get("seed")

		result, err := service.GetMatchupRound(r.Context(), league, seed)
		if err != nil {
			if errors.Is(err, statsservice.ErrNoMatchupRound) {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.MatchupRoundResponse{
			Batter:          result.Batter,
			Prompt:          result.Prompt,
			QuestionType:    result.QuestionType,
			CorrectOpponent: result.CorrectOpponent,
			Options:         result.Options,
			League:          league,
			Metadata: models.MatchupRoundMetadata{
				AvailableLeagues: result.Leagues,
			},
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

type submitDailyDraftRequest struct {
	DeviceID     string   `json:"deviceId"`
	Date         string   `json:"date"`
	Score        float64  `json:"score"`
	OptimalScore float64  `json:"optimalScore"`
	Lineup       []string `json:"lineup"`
}

func SubmitDailyDraftScore(service *gamesservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		var request submitDailyDraftRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request format")
			return
		}

		err := service.SubmitScore(r.Context(), gamesservice.SubmitParams{
			DeviceID:     request.DeviceID,
			League:       league,
			Date:         request.Date,
			Score:        request.Score,
			OptimalScore: request.OptimalScore,
			Lineup:       request.Lineup,
		})
		if err != nil {
			writeDailyDraftError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
		})
	}
}

func GetDailyDraftLeaderboard(service *gamesservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		date := r.URL.Query().Get("date")
		deviceID := r.URL.Query().Get("deviceId")

		result, err := service.Leaderboard(r.Context(), league, date, deviceID)
		if err != nil {
			writeDailyDraftError(w, err)
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"success": true,
			"data":    result,
		})
	}
}

func writeDailyDraftError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, gamesservice.ErrDeviceIDRequired),
		errors.Is(err, gamesservice.ErrDeviceIDTooLong),
		errors.Is(err, gamesservice.ErrDateRequired),
		errors.Is(err, gamesservice.ErrInvalidDate),
		errors.Is(err, gamesservice.ErrInvalidScore),
		errors.Is(err, gamesservice.ErrLineupRequired),
		errors.Is(err, gamesservice.ErrLineupTooLarge):
		writeError(w, http.StatusBadRequest, err.Error())
	case errors.Is(err, gamesservice.ErrAlreadySubmitted):
		writeError(w, http.StatusConflict, err.Error())
	default:
		writeError(w, http.StatusInternalServerError, err.Error())
	}
}
