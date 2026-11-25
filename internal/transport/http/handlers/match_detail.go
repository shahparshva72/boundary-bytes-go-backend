package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
	advancedstatsservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/advancedstats"
)

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
