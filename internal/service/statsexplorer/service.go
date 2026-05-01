package statsexplorer

import (
	"context"
	"errors"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

var (
	ErrReportTypeRequired         = errors.New("reportType is required")
	ErrInvalidReportType          = errors.New("Invalid reportType")
	ErrInvalidDimensions          = errors.New("dimensions must contain between 1 and 3 values")
	ErrInvalidMetrics             = errors.New("metrics must contain between 1 and 8 values")
	ErrTooManySeasons             = errors.New("Maximum 10 seasons can be filtered")
	ErrInvalidSortDirection       = errors.New("sort.direction must be asc or desc")
	ErrInvalidPagination          = errors.New("Invalid pagination")
	ErrDisallowedDimensions       = errors.New("One or more dimensions are not allowed for the selected reportType")
	ErrDisallowedMetrics          = errors.New("One or more metrics are not allowed for the selected reportType")
	ErrInvalidBattingPositions    = errors.New("battingPositions must contain values between 1 and 11")
	ErrDisallowedBattingPositions = errors.New("battingPositions filter is only allowed for batting reports")
)

type Repository interface {
	GetAllLeagues(ctx context.Context) ([]string, error)
	GetStatExplorerOptions(ctx context.Context, league, reportType string) (models.StatExplorerFilterOptions, error)
	RunStatExplorer(ctx context.Context, league string, request models.StatExplorerRunRequest) (models.StatExplorerRunResult, error)
}

type Service struct {
	repository Repository
}

type RunResult struct {
	Result     models.StatExplorerRunResult
	Request    models.StatExplorerRunRequest
	Leagues    []string
	TotalPages int
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) Options(ctx context.Context, league, reportType string) (models.StatExplorerFilterOptions, error) {
	if reportType == "" {
		reportType = "batting"
	}
	if !IsValidReportType(reportType) {
		return models.StatExplorerFilterOptions{}, ErrInvalidReportType
	}
	return s.repository.GetStatExplorerOptions(ctx, league, reportType)
}

func (s *Service) Run(ctx context.Context, league string, request models.StatExplorerRunRequest) (*RunResult, error) {
	if err := normalizeAndValidate(&request); err != nil {
		return nil, err
	}

	result, err := s.repository.RunStatExplorer(ctx, league, request)
	if err != nil {
		return nil, err
	}

	return &RunResult{
		Result:     result,
		Request:    request,
		Leagues:    s.availableLeagues(ctx),
		TotalPages: totalPages(result.TotalRows, request.Pagination.PageSize),
	}, nil
}

func normalizeAndValidate(request *models.StatExplorerRunRequest) error {
	if request.ReportType == "" {
		return ErrReportTypeRequired
	}
	if !IsValidReportType(request.ReportType) {
		return ErrInvalidReportType
	}
	if len(request.Dimensions) < 1 || len(request.Dimensions) > 3 {
		return ErrInvalidDimensions
	}
	if len(request.Metrics) < 1 || len(request.Metrics) > 8 {
		return ErrInvalidMetrics
	}
	if len(request.Filters.Seasons) > 10 {
		return ErrTooManySeasons
	}
	if len(request.Filters.BattingPositions) > 0 {
		if request.ReportType != "batting" {
			return ErrDisallowedBattingPositions
		}
		for _, position := range request.Filters.BattingPositions {
			if position < 1 || position > 11 {
				return ErrInvalidBattingPositions
			}
		}
	}
	if request.Sort != nil && request.Sort.Direction != "" && request.Sort.Direction != "asc" && request.Sort.Direction != "desc" {
		return ErrInvalidSortDirection
	}
	if request.Pagination.Page == 0 {
		request.Pagination.Page = 1
	}
	if request.Pagination.PageSize == 0 {
		request.Pagination.PageSize = 50
	}
	if request.Pagination.Page < 1 || request.Pagination.PageSize < 1 || request.Pagination.PageSize > 200 {
		return ErrInvalidPagination
	}
	if !ValidateDimensions(request.ReportType, request.Dimensions) {
		return ErrDisallowedDimensions
	}
	if !ValidateMetrics(request.ReportType, request.Metrics) {
		return ErrDisallowedMetrics
	}
	return nil
}

func IsValidReportType(reportType string) bool {
	switch reportType {
	case "batting", "bowling", "team", "match":
		return true
	default:
		return false
	}
}

func ValidateDimensions(reportType string, dimensions []string) bool {
	allowed := map[string]map[string]bool{
		"batting": {
			"season": true, "player": true, "team": true, "opposition": true, "venue": true, "city": true, "tossWinner": true, "tossDecision": true, "result": true, "date": true, "innings": true, "battingHand": true, "bowlingType": true, "bowlingSubType": true, "opponentBattingHand": true, "opponentBowlingType": true, "opponentBowlingSubType": true, "playingRole": true, "battingPosition": true,
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

func ValidateMetrics(reportType string, metrics []string) bool {
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

func (s *Service) availableLeagues(ctx context.Context) []string {
	leagues, err := s.repository.GetAllLeagues(ctx)
	if err != nil || leagues == nil {
		return []string{}
	}
	return leagues
}

func totalPages(totalCount, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (totalCount + limit - 1) / limit
}
