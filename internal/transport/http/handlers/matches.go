package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	matchesservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/matches"
)

func GetSeasons(service *matchesservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		result, err := service.GetSeasons(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.SeasonsResponse{
			Seasons: result.Seasons,
			League:  league,
			Metadata: models.SeasonsMetadata{
				AvailableLeagues: result.Leagues,
				TotalSeasons:     len(result.Seasons),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetLatestMatchDate(service *matchesservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		latestDate, err := service.GetLatestMatchDate(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var latestDateValue *string
		if latestDate != nil {
			formatted := latestDate.UTC().Format("2006-01-02T15:04:05.000Z")
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

func GetMatchList(service *matchesservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		result, err := service.GetMatchList(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.MatchListResponse{
			Data:   result.Items,
			League: league,
			Metadata: models.MatchListMetadata{
				AvailableLeagues: result.Leagues,
				TotalRecords:     len(result.Items),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetMatches(service *matchesservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
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

		result, err := service.GetMatches(r.Context(), league, seasonPtr, page, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.MatchesResponse{
			Matches: result.Matches,
			League:  league,
			Pagination: models.Pagination{
				Total:       result.TotalCount,
				Pages:       result.TotalPages,
				CurrentPage: page,
				Limit:       limit,
			},
			Seasons: result.Seasons,
			Metadata: models.MatchesMetadata{
				AvailableLeagues: result.Leagues,
				TotalRecords:     result.TotalCount,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetTeamWins(service *matchesservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		result, err := service.GetTeamWins(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.TeamWinsResponse{
			Data:   result.Items,
			League: league,
			Metadata: models.TeamWinsMetadata{
				AvailableLeagues: result.Leagues,
				TotalTeams:       len(result.Items),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetTeamAverages(service *matchesservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		result, err := service.GetTeamAverages(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.TeamAveragesResponse{
			Data:   result.Items,
			League: league,
			Metadata: models.TeamAveragesMetadata{
				AvailableLeagues: result.Leagues,
				TotalTeams:       len(result.Items),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
