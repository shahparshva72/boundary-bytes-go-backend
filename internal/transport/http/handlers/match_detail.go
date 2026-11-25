package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	advancedstatsservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/advancedstats"
)

func queryInt(r *http.Request, key string, fallback, min, max int) int {
	value, err := strconv.Atoi(r.URL.Query().Get(key))
	if err != nil || value < min {
		return fallback
	}
	if value > max {
		return max
	}
	return value
}

func GetMatchDetail(service *advancedstatsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		matchID, err := strconv.Atoi(chi.URLParam(r, "matchId"))
		if err != nil {
			writeError(w, http.StatusBadRequest, "Invalid match ID")
			return
		}

		scope := strings.Join(leagues.Default().VisibleCodes(""), ",")
		data, err := service.GetMatchDetail(r.Context(), scope, matchID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if data == nil {
			writeError(w, http.StatusNotFound, "Match not found")
			return
		}

		writeJSON(w, http.StatusOK, data)
	}
}

func GetVenueStats(service *advancedstatsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}
		page := queryInt(r, "page", 1, 1, 100000)
		limit := queryInt(r, "limit", 10, 1, 100)
		since := queryInt(r, "since", 0, 0, 3000)
		minMatches := queryInt(r, "minMatches", 5, 1, 1000)

		items, total, pages, err := service.GetVenueStats(r.Context(), league, since, minMatches, r.URL.Query().Get("sort"), page, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, models.VenueStatsResponse{
			Data:       items,
			Pagination: models.Pagination{Total: total, Pages: pages, CurrentPage: page, Limit: limit},
			League:     league,
		})
	}
}

func GetLeagueComparison(service *advancedstatsservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}
		since := queryInt(r, "since", 0, 0, 3000)

		items, err := service.GetLeagueComparison(r.Context(), league, since)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, models.LeagueComparisonResponse{Data: items, League: league, Since: since})
	}
}
