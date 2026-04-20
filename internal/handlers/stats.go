package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

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

func GetPlayerCompare(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		playersParam := r.URL.Query().Get("players")
		seasonsParam := r.URL.Query().Get("seasons")
		team := r.URL.Query().Get("team")
		statType := r.URL.Query().Get("statType")
		if statType == "" {
			statType = "both"
		}

		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}
		if playersParam == "" {
			http.Error(w, "Players parameter is required", http.StatusBadRequest)
			return
		}
		if statType != "batting" && statType != "bowling" && statType != "both" {
			http.Error(w, "Invalid statType: must be batting, bowling, or both", http.StatusBadRequest)
			return
		}

		seenPlayers := map[string]struct{}{}
		players := []string{}
		for _, player := range strings.Split(playersParam, ",") {
			trimmed := strings.TrimSpace(player)
			if trimmed == "" {
				continue
			}
			if _, exists := seenPlayers[trimmed]; exists {
				continue
			}
			seenPlayers[trimmed] = struct{}{}
			players = append(players, trimmed)
		}

		if len(players) < 2 {
			http.Error(w, "At least 2 players are required for comparison", http.StatusBadRequest)
			return
		}
		if len(players) > 5 {
			http.Error(w, "Maximum 5 players can be compared", http.StatusBadRequest)
			return
		}

		var seasons []string
		if seasonsParam != "" {
			for _, season := range strings.Split(seasonsParam, ",") {
				trimmed := strings.TrimSpace(season)
				if trimmed != "" {
					seasons = append(seasons, trimmed)
				}
			}
		}
		if len(seasons) > 10 {
			http.Error(w, "Maximum 10 seasons can be filtered", http.StatusBadRequest)
			return
		}

		var teamPtr *string
		if team != "" {
			teamPtr = &team
		}

		comparedPlayers, err := db.GetPlayerCompare(r.Context(), league, players, seasons, teamPtr, statType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		var seasonsValue []string
		if len(seasons) > 0 {
			seasonsValue = seasons
		}

		resp := models.PlayerCompareResponse{
			Data: models.PlayerCompareData{
				Players: comparedPlayers,
				Filters: models.PlayerCompareFilters{
					Seasons:  seasonsValue,
					Team:     teamPtr,
					StatType: statType,
				},
			},
			League: league,
			Metadata: models.PlayerCompareMetadata{
				AvailableLeagues: leagues,
				PlayerCount:      len(players),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetRunRateTrend(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}

		team := r.URL.Query().Get("team")
		var teamPtr *string
		if team != "" {
			teamPtr = &team
		}

		data, err := db.GetRunRateTrend(r.Context(), league, teamPtr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		resp := models.RunRateTrendResponse{
			Data:   data,
			Team:   teamPtr,
			League: league,
			Metadata: models.RunRateTrendMetadata{
				AvailableLeagues: leagues,
				TotalSeasons:     len(data),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetTeamRunRateProgression(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		team := r.URL.Query().Get("team")
		season := r.URL.Query().Get("season")

		if league == "" || team == "" || season == "" {
			http.Error(w, "league, team, and season parameters are required", http.StatusBadRequest)
			return
		}

		data, metadata, err := db.GetTeamRunRateProgression(r.Context(), league, team, season)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}
		metadata.AvailableLeagues = leagues

		resp := models.TeamRunRateProgressionResponse{
			Data:     data,
			Team:     team,
			Season:   season,
			League:   league,
			Metadata: metadata,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
