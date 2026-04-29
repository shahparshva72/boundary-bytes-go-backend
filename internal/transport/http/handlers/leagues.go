package handlers

import (
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/service/leagues"
)

func GetLeagueConfigs(service *leagues.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configs, err := service.GetConfigs(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, models.LeagueConfigsResponse{Data: configs})
	}
}
