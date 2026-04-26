package handlers

import (
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/database"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func GetLeagueConfigs(db database.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		configs, err := db.GetLeagueConfigStats(r.Context())
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if configs == nil {
			configs = []models.LeagueConfigItem{}
		}

		writeJSON(w, http.StatusOK, models.LeagueConfigsResponse{Data: configs})
	}
}
