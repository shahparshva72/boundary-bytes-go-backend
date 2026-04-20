package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func GetStatExplorerOptions(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		reportType := r.URL.Query().Get("reportType")
		if reportType == "" {
			reportType = "batting"
		}

		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}
		if !isValidStatExplorerReportType(reportType) {
			http.Error(w, "Invalid reportType", http.StatusBadRequest)
			return
		}

		options, err := db.GetStatExplorerOptions(r.Context(), league, reportType)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		resp := models.StatExplorerOptionsResponse{
			Options: options,
			League:  league,
			Metadata: models.StatExplorerOptionsMetadata{
				ReportType: reportType,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func RunStatExplorer(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league := r.URL.Query().Get("league")
		if league == "" {
			http.Error(w, "league parameter is required", http.StatusBadRequest)
			return
		}

		var request models.StatExplorerRunRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if request.ReportType == "" {
			http.Error(w, "reportType is required", http.StatusBadRequest)
			return
		}
		if !isValidStatExplorerReportType(request.ReportType) {
			http.Error(w, "Invalid reportType", http.StatusBadRequest)
			return
		}
		if len(request.Dimensions) < 1 || len(request.Dimensions) > 3 {
			http.Error(w, "dimensions must contain between 1 and 3 values", http.StatusBadRequest)
			return
		}
		if len(request.Metrics) < 1 || len(request.Metrics) > 8 {
			http.Error(w, "metrics must contain between 1 and 8 values", http.StatusBadRequest)
			return
		}
		if len(request.Filters.Seasons) > 10 {
			http.Error(w, "Maximum 10 seasons can be filtered", http.StatusBadRequest)
			return
		}
		if request.Sort != nil && request.Sort.Direction != "" && request.Sort.Direction != "asc" && request.Sort.Direction != "desc" {
			http.Error(w, "sort.direction must be asc or desc", http.StatusBadRequest)
			return
		}
		if request.Pagination.Page == 0 {
			request.Pagination.Page = 1
		}
		if request.Pagination.PageSize == 0 {
			request.Pagination.PageSize = 50
		}
		if request.Pagination.Page < 1 || request.Pagination.PageSize < 1 || request.Pagination.PageSize > 200 {
			http.Error(w, "Invalid pagination", http.StatusBadRequest)
			return
		}
		if !validateStatExplorerDimensions(request.ReportType, request.Dimensions) {
			http.Error(w, "One or more dimensions are not allowed for the selected reportType", http.StatusBadRequest)
			return
		}
		if !validateStatExplorerMetrics(request.ReportType, request.Metrics) {
			http.Error(w, "One or more metrics are not allowed for the selected reportType", http.StatusBadRequest)
			return
		}

		result, err := db.RunStatExplorer(r.Context(), league, request)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		leagues, _ := db.GetAllLeagues(r.Context())
		if leagues == nil {
			leagues = []string{}
		}

		totalPages := 0
		if request.Pagination.PageSize > 0 {
			totalPages = (result.TotalRows + request.Pagination.PageSize - 1) / request.Pagination.PageSize
		}

		resp := models.StatExplorerRunResponse{
			Data:       result.Data,
			Columns:    result.Columns,
			TotalRows:  result.TotalRows,
			Page:       request.Pagination.Page,
			PageSize:   request.Pagination.PageSize,
			TotalPages: totalPages,
			League:     league,
			Metadata: models.StatExplorerRunMetadata{
				AvailableLeagues: leagues,
				ReportType:       request.ReportType,
				Filters:          request.Filters,
				Dimensions:       request.Dimensions,
				Metrics:          request.Metrics,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func isValidStatExplorerReportType(reportType string) bool {
	switch reportType {
	case "batting", "bowling", "team", "match":
		return true
	default:
		return false
	}
}

func validateStatExplorerDimensions(reportType string, dimensions []string) bool {
	allowed := map[string]map[string]bool{
		"batting": {
			"season": true, "player": true, "team": true, "opposition": true, "venue": true, "city": true, "tossWinner": true, "tossDecision": true, "result": true, "date": true, "innings": true, "battingHand": true, "bowlingType": true, "bowlingSubType": true, "opponentBattingHand": true, "opponentBowlingType": true, "opponentBowlingSubType": true, "playingRole": true,
		},
		"bowling": {
			"season": true, "player": true, "team": true, "opposition": true, "venue": true, "city": true, "tossWinner": true, "tossDecision": true, "result": true, "date": true, "innings": true, "battingHand": true, "bowlingType": true, "bowlingSubType": true, "opponentBattingHand": true, "opponentBowlingType": true, "opponentBowlingSubType": true, "playingRole": true,
		},
		"team": {
			"season": true, "team": true, "venue": true, "city": true, "tossWinner": true, "tossDecision": true, "result": true, "date": true,
		},
		"match": {
			"season": true, "team": true, "opposition": true, "venue": true, "city": true, "tossWinner": true, "tossDecision": true, "result": true, "date": true, "innings": true,
		},
	}
	for _, dimension := range dimensions {
		if !allowed[reportType][dimension] {
			return false
		}
	}
	return true
}

func validateStatExplorerMetrics(reportType string, metrics []string) bool {
	allowed := map[string]map[string]bool{
		"batting": {"runs": true, "ballsFaced": true, "innings": true, "notOuts": true, "highestScore": true, "fours": true, "sixes": true, "fifties": true, "hundreds": true, "strikeRate": true, "average": true, "dismissals": true, "dotBalls": true},
		"bowling": {"wickets": true, "ballsBowled": true, "runsConceded": true, "innings": true, "economyRate": true, "bowlingAverage": true, "bowlingStrikeRate": true, "fourWickets": true, "fiveWickets": true, "dotBalls": true},
		"team":    {"matchesPlayed": true, "wins": true, "losses": true, "winPct": true, "winsBattingFirst": true, "winsBattingSecond": true},
		"match":   {"matches": true, "runs": true, "wickets": true, "ballsFaced": true, "ballsBowled": true, "economyRate": true, "strikeRate": true},
	}
	for _, metric := range metrics {
		if !allowed[reportType][metric] {
			return false
		}
	}
	return true
}
