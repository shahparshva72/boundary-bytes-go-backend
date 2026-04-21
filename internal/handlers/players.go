package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func GetBatters(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		batters, err := db.GetBattersByLeague(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		resp := models.PlayerListResponse{
			Data:   batters,
			League: league,
			Metadata: models.PlayerListMetadata{
				AvailableLeagues: leagues,
				TotalRecords:     len(batters),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetBowlers(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		bowlers, err := db.GetBowlersByLeague(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		resp := models.PlayerListResponse{
			Data:   bowlers,
			League: league,
			Metadata: models.PlayerListMetadata{
				AvailableLeagues: leagues,
				TotalRecords:     len(bowlers),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
