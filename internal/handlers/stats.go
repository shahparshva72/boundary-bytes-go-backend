package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func GetMatchup(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		batter := r.URL.Query().Get("batter")
		bowler := r.URL.Query().Get("bowler")

		if league == "" || batter == "" || bowler == "" {
			http.Error(w, "league, batter, and bowler parameters are required", http.StatusBadRequest)
			return
		}

		stats, err := db.GetMatchupStats(r.Context(), league, batter, bowler)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		matchupExists := stats.BallsFaced > 0

		resp := models.MatchupResponse{
			Data:   *stats,
			League: league,
			Batter: batter,
			Bowler: bowler,
			Metadata: models.MatchupMetadata{
				AvailableLeagues: leagues,
				MatchupExists:    matchupExists,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
