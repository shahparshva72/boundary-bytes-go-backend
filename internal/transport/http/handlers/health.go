package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/service/health"
)

func HealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("OK"))
}

func DBHealthCheck(service *health.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		health := service.DBHealth()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(health)
	}
}
