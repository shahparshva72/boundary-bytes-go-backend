package stats

import (
	"context"
	"errors"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

var ErrInvalidBattingPosition = errors.New("batting position must be between 1 and 11")

type Repository interface {
	GetAllLeagues(ctx context.Context) ([]string, error)
	GetLeadingRunScorers(ctx context.Context, league string, page, limit int, battingPositions []int) ([]models.RunScorer, int, error)
	GetLeadingWicketTakers(ctx context.Context, league string, page, limit int) ([]models.WicketTaker, int, error)
	GetMatchupStats(ctx context.Context, league, batter, bowler string) (*models.MatchupData, error)
	GetPlayerCompare(ctx context.Context, league string, players []string, seasons []string, team *string, statType string) ([]models.PlayerComparePlayer, error)
	GetRunRateTrend(ctx context.Context, league string, team *string) ([]models.RunRateTrendItem, error)
	GetTeamRunRateProgression(ctx context.Context, league, team, season string) ([]models.TeamRunRateProgressionPoint, models.TeamRunRateProgressionMetadata, error)
}

type Service struct {
	repository Repository
}

type MatchupResult struct {
	Data          *models.MatchupData
	Leagues       []string
	MatchupExists bool
}

type PaginatedWicketTakersResult struct {
	Items      []models.WicketTaker
	TotalCount int
	TotalPages int
	Leagues    []string
}

type PaginatedRunScorersResult struct {
	Items      []models.RunScorer
	TotalCount int
	TotalPages int
	Leagues    []string
}

type PlayerCompareResult struct {
	Players []models.PlayerComparePlayer
	Leagues []string
}

type RunRateTrendResult struct {
	Items   []models.RunRateTrendItem
	Leagues []string
}

type TeamRunRateProgressionResult struct {
	Items    []models.TeamRunRateProgressionPoint
	Metadata models.TeamRunRateProgressionMetadata
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetMatchup(ctx context.Context, league, batter, bowler string) (*MatchupResult, error) {
	data, err := s.repository.GetMatchupStats(ctx, league, batter, bowler)
	if err != nil {
		return nil, err
	}
	return &MatchupResult{
		Data:          data,
		Leagues:       s.availableLeagues(ctx),
		MatchupExists: data != nil && data.BallsFaced > 0,
	}, nil
}

func (s *Service) GetLeadingWicketTakers(ctx context.Context, league string, page, limit int) (*PaginatedWicketTakersResult, error) {
	items, totalCount, err := s.repository.GetLeadingWicketTakers(ctx, league, page, limit)
	if err != nil {
		return nil, err
	}
	return &PaginatedWicketTakersResult{
		Items:      items,
		TotalCount: totalCount,
		TotalPages: totalPages(totalCount, limit),
		Leagues:    s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetLeadingRunScorers(ctx context.Context, league string, page, limit int, battingPositions []int) (*PaginatedRunScorersResult, error) {
	if err := validateBattingPositions(battingPositions); err != nil {
		return nil, err
	}

	items, totalCount, err := s.repository.GetLeadingRunScorers(ctx, league, page, limit, battingPositions)
	if err != nil {
		return nil, err
	}
	return &PaginatedRunScorersResult{
		Items:      items,
		TotalCount: totalCount,
		TotalPages: totalPages(totalCount, limit),
		Leagues:    s.availableLeagues(ctx),
	}, nil
}

func validateBattingPositions(positions []int) error {
	for _, position := range positions {
		if position < 1 || position > 11 {
			return ErrInvalidBattingPosition
		}
	}
	return nil
}

func (s *Service) GetPlayerCompare(ctx context.Context, league string, players []string, seasons []string, team *string, statType string) (*PlayerCompareResult, error) {
	comparedPlayers, err := s.repository.GetPlayerCompare(ctx, league, players, seasons, team, statType)
	if err != nil {
		return nil, err
	}
	return &PlayerCompareResult{
		Players: comparedPlayers,
		Leagues: s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetRunRateTrend(ctx context.Context, league string, team *string) (*RunRateTrendResult, error) {
	items, err := s.repository.GetRunRateTrend(ctx, league, team)
	if err != nil {
		return nil, err
	}
	return &RunRateTrendResult{
		Items:   items,
		Leagues: s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetTeamRunRateProgression(ctx context.Context, league, team, season string) (*TeamRunRateProgressionResult, error) {
	items, metadata, err := s.repository.GetTeamRunRateProgression(ctx, league, team, season)
	if err != nil {
		return nil, err
	}
	metadata.AvailableLeagues = s.availableLeagues(ctx)
	return &TeamRunRateProgressionResult{Items: items, Metadata: metadata}, nil
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
