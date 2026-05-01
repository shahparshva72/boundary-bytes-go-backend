package matches

import (
	"context"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

type Repository interface {
	GetAllLeagues(ctx context.Context) ([]string, error)
	GetLatestMatchDate(ctx context.Context, league string) (*time.Time, error)
	GetMatchList(ctx context.Context, league string) ([]models.MatchListItem, error)
	GetMatches(ctx context.Context, league string, season *string, page, limit int) ([]models.MatchCard, int, []string, error)
	GetSeasonsByLeague(ctx context.Context, league string) ([]string, error)
	GetTeamAverages(ctx context.Context, league string) ([]models.TeamAverageItem, error)
	GetTeamWins(ctx context.Context, league string) ([]models.TeamWinsItem, error)
}

type Service struct {
	repository Repository
}

type SeasonsResult struct {
	Seasons []string
	Leagues []string
}

type MatchListResult struct {
	Items   []models.MatchListItem
	Leagues []string
}

type MatchesResult struct {
	Matches    []models.MatchCard
	TotalCount int
	TotalPages int
	Seasons    []string
	Leagues    []string
}

type TeamWinsResult struct {
	Items   []models.TeamWinsItem
	Leagues []string
}

type TeamAveragesResult struct {
	Items   []models.TeamAverageItem
	Leagues []string
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetSeasons(ctx context.Context, league string) (*SeasonsResult, error) {
	seasons, err := s.repository.GetSeasonsByLeague(ctx, league)
	if err != nil {
		return nil, err
	}

	return &SeasonsResult{
		Seasons: seasons,
		Leagues: s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetLatestMatchDate(ctx context.Context, league string) (*time.Time, error) {
	return s.repository.GetLatestMatchDate(ctx, league)
}

func (s *Service) GetMatchList(ctx context.Context, league string) (*MatchListResult, error) {
	items, err := s.repository.GetMatchList(ctx, league)
	if err != nil {
		return nil, err
	}

	return &MatchListResult{
		Items:   items,
		Leagues: s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetMatches(ctx context.Context, league string, season *string, page, limit int) (*MatchesResult, error) {
	matches, totalCount, seasons, err := s.repository.GetMatches(ctx, league, season, page, limit)
	if err != nil {
		return nil, err
	}

	return &MatchesResult{
		Matches:    matches,
		TotalCount: totalCount,
		TotalPages: totalPages(totalCount, limit),
		Seasons:    seasons,
		Leagues:    s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetTeamWins(ctx context.Context, league string) (*TeamWinsResult, error) {
	items, err := s.repository.GetTeamWins(ctx, league)
	if err != nil {
		return nil, err
	}

	return &TeamWinsResult{
		Items:   items,
		Leagues: s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetTeamAverages(ctx context.Context, league string) (*TeamAveragesResult, error) {
	items, err := s.repository.GetTeamAverages(ctx, league)
	if err != nil {
		return nil, err
	}

	return &TeamAveragesResult{
		Items:   items,
		Leagues: s.availableLeagues(ctx),
	}, nil
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
