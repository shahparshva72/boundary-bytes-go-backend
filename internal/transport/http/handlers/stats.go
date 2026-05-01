package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	statsservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/stats"
)

func GetMatchup(service *statsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}
		batter := r.URL.Query().Get("batter")
		bowler := r.URL.Query().Get("bowler")

		if batter == "" || bowler == "" {
			writeError(w, http.StatusBadRequest, "batter and bowler parameters are required")
			return
		}

		result, err := service.GetMatchup(r.Context(), league, batter, bowler)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.MatchupResponse{
			Data:   *result.Data,
			League: league,
			Batter: batter,
			Bowler: bowler,
			Metadata: models.MatchupMetadata{
				AvailableLeagues: result.Leagues,
				MatchupExists:    result.MatchupExists,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetLeadingWicketTakers(service *statsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
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

		result, err := service.GetLeadingWicketTakers(r.Context(), league, page, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.LeadingWicketTakersResponse{
			League: league,
			Pagination: models.Pagination{
				Total:       result.TotalCount,
				Pages:       result.TotalPages,
				CurrentPage: page,
				Limit:       limit,
			},
			Metadata: models.LeadingWicketTakersMetadata{
				AvailableLeagues: result.Leagues,
				TotalRecords:     result.TotalCount,
			},
			Data: result.Items,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetLeadingRunScorers(service *statsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
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

		battingPositions, err := parseBattingPositions(r)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}

		result, err := service.GetLeadingRunScorers(r.Context(), league, page, limit, battingPositions)
		if err != nil {
			if err == statsservice.ErrInvalidBattingPosition {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.LeadingRunScorersResponse{
			Data:   result.Items,
			League: league,
			Pagination: models.Pagination{
				Total:       result.TotalCount,
				Pages:       result.TotalPages,
				CurrentPage: page,
				Limit:       limit,
			},
			Metadata: models.LeadingRunScorersMetadata{
				AvailableLeagues: result.Leagues,
				TotalRecords:     result.TotalCount,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func parseBattingPositions(r *http.Request) ([]int, error) {
	seen := map[int]struct{}{}
	positions := []int{}
	appendPosition := func(raw string) error {
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" {
			return nil
		}
		position, err := strconv.Atoi(trimmed)
		if err != nil || position < 1 || position > 11 {
			return statsservice.ErrInvalidBattingPosition
		}
		if _, exists := seen[position]; exists {
			return nil
		}
		seen[position] = struct{}{}
		positions = append(positions, position)
		return nil
	}

	if single := r.URL.Query().Get("battingPosition"); single != "" {
		if err := appendPosition(single); err != nil {
			return nil, err
		}
	}
	if multi := r.URL.Query().Get("battingPositions"); multi != "" {
		for _, raw := range strings.Split(multi, ",") {
			if err := appendPosition(raw); err != nil {
				return nil, err
			}
		}
	}

	return positions, nil
}

func GetPlayerCompare(service *statsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}
		playersParam := r.URL.Query().Get("players")
		seasonsParam := r.URL.Query().Get("seasons")
		team := r.URL.Query().Get("team")
		statType := r.URL.Query().Get("statType")
		if statType == "" {
			statType = "both"
		}

		if playersParam == "" {
			writeError(w, http.StatusBadRequest, "Players parameter is required")
			return
		}
		if statType != "batting" && statType != "bowling" && statType != "both" {
			writeError(w, http.StatusBadRequest, "Invalid statType: must be batting, bowling, or both")
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
			writeError(w, http.StatusBadRequest, "At least 2 players are required for comparison")
			return
		}
		if len(players) > 5 {
			writeError(w, http.StatusBadRequest, "Maximum 5 players can be compared")
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
			writeError(w, http.StatusBadRequest, "Maximum 10 seasons can be filtered")
			return
		}

		var teamPtr *string
		if team != "" {
			teamPtr = &team
		}

		result, err := service.GetPlayerCompare(r.Context(), league, players, seasons, teamPtr, statType)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var seasonsValue []string
		if len(seasons) > 0 {
			seasonsValue = seasons
		}

		resp := models.PlayerCompareResponse{
			Data: models.PlayerCompareData{
				Players: result.Players,
				Filters: models.PlayerCompareFilters{
					Seasons:  seasonsValue,
					Team:     teamPtr,
					StatType: statType,
				},
			},
			League: league,
			Metadata: models.PlayerCompareMetadata{
				AvailableLeagues: result.Leagues,
				PlayerCount:      len(players),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetRunRateTrend(service *statsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		team := r.URL.Query().Get("team")
		var teamPtr *string
		if team != "" {
			teamPtr = &team
		}

		result, err := service.GetRunRateTrend(r.Context(), league, teamPtr)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.RunRateTrendResponse{
			Data:   result.Items,
			Team:   teamPtr,
			League: league,
			Metadata: models.RunRateTrendMetadata{
				AvailableLeagues: result.Leagues,
				TotalSeasons:     len(result.Items),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetTeamRunRateProgression(service *statsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}
		team := r.URL.Query().Get("team")
		season := r.URL.Query().Get("season")

		if team == "" || season == "" {
			writeError(w, http.StatusBadRequest, "team and season parameters are required")
			return
		}

		result, err := service.GetTeamRunRateProgression(r.Context(), league, team, season)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.TeamRunRateProgressionResponse{
			Data:     result.Items,
			Team:     team,
			Season:   season,
			League:   league,
			Metadata: result.Metadata,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
