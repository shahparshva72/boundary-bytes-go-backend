package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/service/players"
)

func GetBatters(service *players.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		result, err := service.GetBatters(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.PlayerListResponse{
			Data:   result.Players,
			League: league,
			Metadata: models.PlayerListMetadata{
				AvailableLeagues: result.Leagues,
				TotalRecords:     len(result.Players),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetBowlers(service *players.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		result, err := service.GetBowlers(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.PlayerListResponse{
			Data:   result.Players,
			League: league,
			Metadata: models.PlayerListMetadata{
				AvailableLeagues: result.Leagues,
				TotalRecords:     len(result.Players),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
