package statsexplorer

import (
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func TestNormalizeAndValidateAppliesPaginationDefaults(t *testing.T) {
	request := models.StatExplorerRunRequest{
		ReportType: "batting",
		Dimensions: []string{"player"},
		Metrics:    []string{"runs"},
	}

	if err := normalizeAndValidate(&request); err != nil {
		t.Fatalf("normalizeAndValidate() error = %v", err)
	}
	if request.Pagination.Page != 1 {
		t.Fatalf("Page = %d, want 1", request.Pagination.Page)
	}
	if request.Pagination.PageSize != 50 {
		t.Fatalf("PageSize = %d, want 50", request.Pagination.PageSize)
	}
}

func TestNormalizeAndValidateRejectsDisallowedMetric(t *testing.T) {
	request := models.StatExplorerRunRequest{
		ReportType: "team",
		Dimensions: []string{"team"},
		Metrics:    []string{"runs"},
	}

	if err := normalizeAndValidate(&request); err != ErrDisallowedMetrics {
		t.Fatalf("normalizeAndValidate() error = %v, want %v", err, ErrDisallowedMetrics)
	}
}
