package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	statsexplorerservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/statsexplorer"
)

func GetStatExplorerOptions(service *statsexplorerservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}
		reportType := r.URL.Query().Get("reportType")
		if reportType == "" {
			reportType = "batting"
		}

		options, err := service.Options(r.Context(), league, reportType)
		if err != nil {
			if errors.Is(err, statsexplorerservice.ErrInvalidReportType) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
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

func RunStatExplorer(service *statsexplorerservice.Service) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		league, ok := resolveLeague(w, r)
		if !ok {
			return
		}

		var request models.StatExplorerRunRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		result, err := service.Run(r.Context(), league, request)
		if err != nil {
			if isStatExplorerValidationError(err) {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		resp := models.StatExplorerRunResponse{
			Data:       result.Result.Data,
			Columns:    result.Result.Columns,
			TotalRows:  result.Result.TotalRows,
			Page:       result.Request.Pagination.Page,
			PageSize:   result.Request.Pagination.PageSize,
			TotalPages: result.TotalPages,
			League:     league,
			Metadata: models.StatExplorerRunMetadata{
				AvailableLeagues: result.Leagues,
				ReportType:       result.Request.ReportType,
				Filters:          result.Request.Filters,
				Dimensions:       result.Request.Dimensions,
				Metrics:          result.Request.Metrics,
			},
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}

func isStatExplorerValidationError(err error) bool {
	return errors.Is(err, statsexplorerservice.ErrReportTypeRequired) ||
		errors.Is(err, statsexplorerservice.ErrInvalidReportType) ||
		errors.Is(err, statsexplorerservice.ErrInvalidDimensions) ||
		errors.Is(err, statsexplorerservice.ErrInvalidMetrics) ||
		errors.Is(err, statsexplorerservice.ErrTooManySeasons) ||
		errors.Is(err, statsexplorerservice.ErrInvalidSortDirection) ||
		errors.Is(err, statsexplorerservice.ErrInvalidPagination) ||
		errors.Is(err, statsexplorerservice.ErrDisallowedDimensions) ||
		errors.Is(err, statsexplorerservice.ErrDisallowedMetrics)
}
