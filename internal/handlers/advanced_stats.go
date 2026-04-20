package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func GetBowlingWicketTypes(db database.Service) http.HandlerFunc {
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

		limit := 10
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		data, totalCount, err := db.GetBowlingWicketTypes(r.Context(), league, page, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		totalPages := (totalCount + limit - 1) / limit

		resp := models.BowlingWicketTypesResponse{
			Data:   data,
			League: league,
			Pagination: models.Pagination{
				Total:       totalCount,
				Pages:       totalPages,
				CurrentPage: page,
				Limit:       limit,
			},
			Metadata: models.BowlingWicketTypesMetadata{
				AvailableLeagues: leagues,
				TotalRecords:     totalCount,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetMultiMatchup(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		player := r.URL.Query().Get("player")
		opponentsParam := r.URL.Query().Get("opponents")
		mode := r.URL.Query().Get("mode")

		if league == "" || player == "" || opponentsParam == "" || mode == "" {
			http.Error(w, "league, player, opponents, and mode parameters are required", http.StatusBadRequest)
			return
		}

		if mode != "batterVsBowlers" && mode != "bowlerVsBatters" {
			http.Error(w, "mode must be either \"batterVsBowlers\" or \"bowlerVsBatters\"", http.StatusBadRequest)
			return
		}

		opponents := strings.Split(opponentsParam, ",")
		for index, opponent := range opponents {
			opponents[index] = strings.TrimSpace(opponent)
		}

		if len(opponents) > 5 {
			http.Error(w, "Maximum 5 opponents allowed", http.StatusBadRequest)
			return
		}

		results, err := db.GetMultiMatchup(r.Context(), league, player, opponents, mode)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		combined := models.MultiMatchupCombined{}
		for _, result := range results {
			combined.RunsScored += result.RunsScored
			combined.BallsFaced += result.BallsFaced
			combined.Dismissals += result.Dismissals
			combined.Fours += result.Fours
			combined.Sixes += result.Sixes
			combined.DotBalls += result.DotBalls
		}

		if combined.BallsFaced > 0 {
			combined.StrikeRate = float64(combined.RunsScored) / float64(combined.BallsFaced) * 100
			combined.EconomyRate = float64(combined.RunsScored) / (float64(combined.BallsFaced) / 6)
		}
		if combined.Dismissals > 0 {
			combined.Average = float64(combined.RunsScored) / float64(combined.Dismissals)
		} else {
			combined.Average = float64(combined.RunsScored)
		}

		combined.StrikeRate = roundToTwo(combined.StrikeRate)
		combined.EconomyRate = roundToTwo(combined.EconomyRate)
		combined.Average = roundToTwo(combined.Average)

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		resp := models.MultiMatchupResponse{
			Data:      results,
			Combined:  combined,
			League:    league,
			Player:    player,
			Mode:      mode,
			Opponents: opponents,
			Metadata: models.MultiMatchupMetadata{
				AvailableLeagues: leagues,
				ResultCount:      len(results),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetPlayerProgression(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		player := r.URL.Query().Get("player")
		inningsParam := r.URL.Query().Get("innings")

		if league == "" || player == "" {
			http.Error(w, "league and player parameters are required", http.StatusBadRequest)
			return
		}

		var inningsPtr *int
		if inningsParam != "" {
			if inningsValue, err := strconv.Atoi(inningsParam); err == nil {
				inningsPtr = &inningsValue
			} else {
				http.Error(w, "Invalid innings parameter", http.StatusBadRequest)
				return
			}
		}

		data, metadata, err := db.GetPlayerProgression(r.Context(), league, player, inningsPtr)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}
		metadata.AvailableLeagues = leagues

		var inningsValue *string
		if inningsParam != "" {
			inningsValue = &inningsParam
		}

		resp := models.PlayerProgressionResponse{
			Data:     data,
			Player:   player,
			League:   league,
			Innings:  inningsValue,
			Metadata: metadata,
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetAdvancedStats(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		oversParam := r.URL.Query().Get("overs")
		batter := r.URL.Query().Get("batter")
		bowler := r.URL.Query().Get("bowler")
		playerType := r.URL.Query().Get("playerType")
		if playerType == "" {
			playerType = "batter"
		}

		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}
		if oversParam == "" {
			http.Error(w, "overs parameter is required", http.StatusBadRequest)
			return
		}
		if batter == "" && bowler == "" {
			http.Error(w, "Either batter or bowler must be specified", http.StatusBadRequest)
			return
		}

		overs := []int{}
		for _, value := range strings.Split(oversParam, ",") {
			over, err := strconv.Atoi(strings.TrimSpace(value))
			if err != nil {
				http.Error(w, "Invalid over numbers provided", http.StatusBadRequest)
				return
			}
			overs = append(overs, over)
		}

		player := batter
		if playerType == "bowler" {
			player = bowler
		}

		data, deliveryCount, err := db.GetAdvancedStats(r.Context(), league, playerType, player, overs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		resp := models.AdvancedStatsResponse{
			Data:       data,
			League:     league,
			Player:     player,
			PlayerType: playerType,
			Overs:      overs,
			Metadata: models.AdvancedStatsMetadata{
				AvailableLeagues:   leagues,
				DeliveriesAnalyzed: deliveryCount,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetFallOfWickets(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		matchParam := chi.URLParam(r, "matchId")

		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}
		matchID, err := strconv.Atoi(matchParam)
		if err != nil {
			http.Error(w, "Invalid match ID", http.StatusBadRequest)
			return
		}

		data, err := db.GetFallOfWickets(r.Context(), league, matchID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if data == nil {
			http.Error(w, "Match not found in league", http.StatusNotFound)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}
		data.Metadata.AvailableLeagues = leagues

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(data)
	}
}

func roundToTwo(value float64) float64 {
	if value == 0 {
		return 0
	}
	return float64(int(value*100+0.5)) / 100
}
