package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/service/players"
)

func GetBatters(service *players.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		result, err := service.GetBatters(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.PlayerListResponse{
			Data:   result.Players,
			League: league,
			Metadata: models.PlayerListMetadata{
				AvailableLeagues: result.Leagues,
				TotalRecords:     len(result.Players),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetBowlers(service *players.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		result, err := service.GetBowlers(r.Context(), league)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.PlayerListResponse{
			Data:   result.Players,
			League: league,
			Metadata: models.PlayerListMetadata{
				AvailableLeagues: result.Leagues,
				TotalRecords:     len(result.Players),
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func GetPlayerProfile(service *players.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := chi.URLParam(r, "slug")
		if slug == "" {
			writeError(w, http.StatusNotFound, "player not found")
			return
		}

		result, err := service.GetPlayerProfile(r.Context(), slug)
		if errors.Is(err, players.ErrNotFound) {
			writeError(w, http.StatusNotFound, "player not found")
			return
		}
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.PlayerProfileResponse{
			Slug:        result.Slug,
			Name:        result.Name,
			Bio:         result.Bio,
			LeagueStats: result.LeagueStats,
			Metadata: models.PlayerProfileMetadata{
				AvailableLeagues: result.Leagues,
			},
		}

		writeJSON(w, http.StatusOK, resp)
	}
}

func ListPlayerSlugs(service *players.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		page := 1
		if pageStr := r.URL.Query().Get("page"); pageStr != "" {
			if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
				page = p
			}
		}

		limit := 50
		if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
			if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
				limit = l
			}
		}

		entries, total, err := service.ListPlayerSlugs(r.Context(), page, limit)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"data": entries,
			"pagination": models.Pagination{
				Total:       total,
				Pages:       (total + limit - 1) / limit,
				CurrentPage: page,
				Limit:       limit,
			},
		})
	}
}
