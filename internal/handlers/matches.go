package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func GetSeasons(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}

		seasons, err := db.GetSeasonsByLeague(r.Context(), league)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		resp := models.SeasonsResponse{
			Seasons: seasons,
			League:  league,
			Metadata: models.SeasonsMetadata{
				AvailableLeagues: leagues,
				TotalSeasons:     len(seasons),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetLatestMatchDate(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}

		latestDate, err := db.GetLatestMatchDate(r.Context(), league)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var latestDateValue *string
		if latestDate != nil {
			formatted := latestDate.UTC().Format(time.RFC3339Nano)
			latestDateValue = &formatted
		}

		resp := models.LatestMatchDateResponse{
			League:     league,
			LatestDate: latestDateValue,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetMatchList(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}

		items, err := db.GetMatchList(r.Context(), league)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		resp := models.MatchListResponse{
			Data:   items,
			League: league,
			Metadata: models.MatchListMetadata{
				AvailableLeagues: leagues,
				TotalRecords:     len(items),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetMatches(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}

		page := 1
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		limit := 5
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		season := r.URL.Query().Get("season")
		var seasonPtr *string
		if season != "" {
			seasonPtr = &season
		}

		matches, totalCount, seasons, err := db.GetMatches(r.Context(), league, seasonPtr, page, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		totalPages := 0
		if limit > 0 {
			totalPages = (totalCount + limit - 1) / limit
		}

		resp := models.MatchesResponse{
			Matches: matches,
			League:  league,
			Pagination: models.Pagination{
				Total:       totalCount,
				Pages:       totalPages,
				CurrentPage: page,
				Limit:       limit,
			},
			Seasons: seasons,
			Metadata: models.MatchesMetadata{
				AvailableLeagues: leagues,
				TotalRecords:     totalCount,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetTeamWins(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}

		data, err := db.GetTeamWins(r.Context(), league)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		resp := models.TeamWinsResponse{
			Data:   data,
			League: league,
			Metadata: models.TeamWinsMetadata{
				AvailableLeagues: leagues,
				TotalTeams:       len(data),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetTeamAverages(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}

		data, err := db.GetTeamAverages(r.Context(), league)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		resp := models.TeamAveragesResponse{
			Data:   data,
			League: league,
			Metadata: models.TeamAveragesMetadata{
				AvailableLeagues: leagues,
				TotalTeams:       len(data),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
