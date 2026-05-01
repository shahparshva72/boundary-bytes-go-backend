package players

import (
	"context"
)

type Repository interface {
	GetAllLeagues(ctx context.Context) ([]string, error)
	GetBattersByLeague(ctx context.Context, league string) ([]string, error)
	GetBowlersByLeague(ctx context.Context, league string) ([]string, error)
}

type Service struct {
	repository Repository
}

type ListResult struct {
	Players []string
	Leagues []string
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetBatters(ctx context.Context, league string) (*ListResult, error) {
	batters, err := s.repository.GetBattersByLeague(ctx, league)
	if err != nil {
		return nil, err
	}

	return &ListResult{
		Players: batters,
		Leagues: s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetBowlers(ctx context.Context, league string) (*ListResult, error) {
	bowlers, err := s.repository.GetBowlersByLeague(ctx, league)
	if err != nil {
		return nil, err
	}

	return &ListResult{
		Players: bowlers,
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
