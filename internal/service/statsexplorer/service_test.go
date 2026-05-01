package statsexplorer

import (
	"context"
	"errors"
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

type statExplorerRepositoryStub struct {
	options models.StatExplorerFilterOptions
	err     error
}

func (s statExplorerRepositoryStub) GetAllLeagues(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (s statExplorerRepositoryStub) GetStatExplorerOptions(
	ctx context.Context,
	league string,
	reportType string,
) (models.StatExplorerFilterOptions, error) {
	return s.options, s.err
}

func (s statExplorerRepositoryStub) RunStatExplorer(
	ctx context.Context,
	league string,
	request models.StatExplorerRunRequest,
) (models.StatExplorerRunResult, error) {
	return models.StatExplorerRunResult{}, nil
}

func TestOptionsKeepsBattingPositionsForBattingReports(t *testing.T) {
	service := New(statExplorerRepositoryStub{
		options: models.StatExplorerFilterOptions{
			BattingPositions: []int{1, 2, 3},
		},
	})

	options, err := service.Options(context.Background(), "wpl", "batting")
	if err != nil {
		t.Fatalf("Options() error = %v", err)
	}
	if len(options.BattingPositions) != 3 {
		t.Fatalf("BattingPositions = %v, want [1 2 3]", options.BattingPositions)
	}
}

func TestOptionsClearsBattingPositionsForNonBattingReports(t *testing.T) {
	for _, reportType := range []string{"bowling", "team", "match"} {
		t.Run(reportType, func(t *testing.T) {
			service := New(statExplorerRepositoryStub{
				options: models.StatExplorerFilterOptions{
					BattingPositions: []int{1, 2, 3},
				},
			})

			options, err := service.Options(context.Background(), "wpl", reportType)
			if err != nil {
				t.Fatalf("Options() error = %v", err)
			}
			if len(options.BattingPositions) != 0 {
				t.Fatalf("BattingPositions = %v, want empty", options.BattingPositions)
			}
		})
	}
}

func TestOptionsReturnsRepositoryError(t *testing.T) {
	want := errors.New("repository failed")
	service := New(statExplorerRepositoryStub{err: want})

	if _, err := service.Options(context.Background(), "wpl", "batting"); err != want {
		t.Fatalf("Options() error = %v, want %v", err, want)
	}
}

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

func TestNormalizeAndValidateAcceptsBattingPositionForBatting(t *testing.T) {
	request := models.StatExplorerRunRequest{
		ReportType: "batting",
		Dimensions: []string{"battingPosition", "player"},
		Metrics:    []string{"runs"},
		Filters: models.StatExplorerRunFilters{
			BattingPositions: []int{3},
		},
	}

	if err := normalizeAndValidate(&request); err != nil {
		t.Fatalf("normalizeAndValidate() error = %v", err)
	}
}

func TestNormalizeAndValidateRejectsBattingPositionForBowling(t *testing.T) {
	request := models.StatExplorerRunRequest{
		ReportType: "bowling",
		Dimensions: []string{"battingPosition"},
		Metrics:    []string{"wickets"},
	}

	if err := normalizeAndValidate(&request); err != ErrDisallowedDimensions {
		t.Fatalf("normalizeAndValidate() error = %v, want %v", err, ErrDisallowedDimensions)
	}
}

func TestNormalizeAndValidateRejectsInvalidBattingPositions(t *testing.T) {
	request := models.StatExplorerRunRequest{
		ReportType: "batting",
		Dimensions: []string{"player"},
		Metrics:    []string{"runs"},
		Filters: models.StatExplorerRunFilters{
			BattingPositions: []int{12},
		},
	}

	if err := normalizeAndValidate(&request); err != ErrInvalidBattingPositions {
		t.Fatalf("normalizeAndValidate() error = %v, want %v", err, ErrInvalidBattingPositions)
	}
}
