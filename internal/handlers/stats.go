package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

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

func GetLeadingWicketTakers(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}

		// Parse page parameter with default 1
		page := 1
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		// Parse limit parameter with default 10
		limit := 10
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		wicketTakers, totalCount, err := db.GetLeadingWicketTakers(r.Context(), league, page, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		// Calculate pagination info
		totalPages := (totalCount + limit - 1) / limit

		resp := models.LeadingWicketTakersResponse{
			League: league,
			Pagination: models.Pagination{
				Total:       totalCount,
				Pages:       totalPages,
				CurrentPage: page,
				Limit:       limit,
			},
			Metadata: models.LeadingWicketTakersMetadata{
				AvailableLeagues: leagues,
				TotalRecords:     totalCount,
			},
			Data: wicketTakers,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetLeadingRunScorers(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}

		// Parse page parameter with default 1
		page := 1
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		// Parse limit parameter with default 10
		limit := 10
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		runScorers, totalCount, err := db.GetLeadingRunScorers(r.Context(), league, page, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		// Calculate pagination info
		totalPages := (totalCount + limit - 1) / limit

		resp := models.LeadingRunScorersResponse{
			Data:   runScorers,
			League: league,
			Pagination: models.Pagination{
				Total:       totalCount,
				Pages:       totalPages,
				CurrentPage: page,
				Limit:       limit,
			},
			Metadata: models.LeadingRunScorersMetadata{
				AvailableLeagues: leagues,
				TotalRecords:     totalCount,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}