package handlers

import (
	"errors"
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
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
