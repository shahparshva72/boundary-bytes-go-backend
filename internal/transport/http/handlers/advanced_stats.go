package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	advancedstatsservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/advancedstats"
)

func GetBowlingWicketTypes(service *advancedstatsservice.Service) http.HandlerFunc {
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

		limit := 10
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		result, err := service.GetBowlingWicketTypes(r.Context(), league, page, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.BowlingWicketTypesResponse{
			Data:   result.Items,
			League: league,
			Pagination: models.Pagination{
				Total:       result.TotalCount,
				Pages:       result.TotalPages,
				CurrentPage: page,
				Limit:       limit,
			},
			Metadata: models.BowlingWicketTypesMetadata{
				AvailableLeagues: result.Leagues,
				TotalRecords:     result.TotalCount,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetMultiMatchup(service *advancedstatsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}
		player := r.URL.Query().Get("player")
		opponentsParam := r.URL.Query().Get("opponents")
		mode := r.URL.Query().Get("mode")

		if player == "" || opponentsParam == "" || mode == "" {
			writeError(w, http.StatusBadRequest, "player, opponents, and mode parameters are required")
			return
		}

		if mode != "batterVsBowlers" && mode != "bowlerVsBatters" {
			writeError(w, http.StatusBadRequest, "mode must be either \"batterVsBowlers\" or \"bowlerVsBatters\"")
			return
		}

		opponents := strings.Split(opponentsParam, ",")
		for index, opponent := range opponents {
			opponents[index] = strings.TrimSpace(opponent)
		}

		if len(opponents) > 5 {
			writeError(w, http.StatusBadRequest, "Maximum 5 opponents allowed")
			return
		}

		result, err := service.GetMultiMatchup(r.Context(), league, player, opponents, mode)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.MultiMatchupResponse{
			Data:      result.Items,
			Combined:  result.Combined,
			League:    league,
			Player:    player,
			Mode:      mode,
			Opponents: opponents,
			Metadata: models.MultiMatchupMetadata{
				AvailableLeagues: result.Leagues,
				ResultCount:      len(result.Items),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetPlayerProgression(service *advancedstatsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}
		player := r.URL.Query().Get("player")
		inningsParam := r.URL.Query().Get("innings")

		if player == "" {
			writeError(w, http.StatusBadRequest, "player parameter is required")
			return
		}

		var inningsPtr *int
		if inningsParam != "" {
			if inningsValue, err := strconv.Atoi(inningsParam); err == nil {
				inningsPtr = &inningsValue
			} else {
				writeError(w, http.StatusBadRequest, "Invalid innings parameter")
				return
			}
		}

		result, err := service.GetPlayerProgression(r.Context(), league, player, inningsPtr)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		var inningsValue *string
		if inningsParam != "" {
			inningsValue = &inningsParam
		}

		resp := models.PlayerProgressionResponse{
			Data:     result.Items,
			Player:   player,
			League:   league,
			Innings:  inningsValue,
			Metadata: result.Metadata,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetAdvancedStats(service *advancedstatsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}
		oversParam := r.URL.Query().Get("overs")
		batter := r.URL.Query().Get("batter")
		bowler := r.URL.Query().Get("bowler")
		playerType := r.URL.Query().Get("playerType")
		if playerType == "" {
			playerType = "batter"
		}

		if oversParam == "" {
			writeError(w, http.StatusBadRequest, "overs parameter is required")
			return
		}
		if batter == "" && bowler == "" {
			writeError(w, http.StatusBadRequest, "Either batter or bowler must be specified")
			return
		}

		overs := []int{}
		for _, value := range strings.Split(oversParam, ",") {
			over, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				writeError(w, http.StatusBadRequest, "Invalid over numbers provided")
				return
			}
			overs = append(overs, over)
		}

		player := batter
		if playerType == "bowler" {
			player = bowler
		}

		result, err := service.GetAdvancedStats(r.Context(), league, playerType, player, overs)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.AdvancedStatsResponse{
			Data:       result.Data,
			League:     league,
			Player:     player,
			PlayerType: playerType,
			Overs:      overs,
			Metadata: models.AdvancedStatsMetadata{
				AvailableLeagues:   result.Leagues,
				DeliveriesAnalyzed: result.DeliveryCount,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetFallOfWickets(service *advancedstatsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}
		matchParam := chi.URLParam(r, "matchId")

		matchID, err := strconv.Atoi(matchParam)
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid match ID")
			return
		}

		data, err := service.GetFallOfWickets(r.Context(), league, matchID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if data == nil {
			writeError(w, http.StatusNotFound, "Match not found in league")
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}
