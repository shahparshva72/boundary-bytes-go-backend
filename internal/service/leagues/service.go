package leagues

import (
	"context"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

type Repository interface {
	GetLeagueConfigStats(ctx context.Context) ([]models.LeagueConfigItem, error)
}

type Service struct {
	repository Repository
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetConfigs(ctx context.Context) ([]models.LeagueConfigItem, error) {
	configs, err := s.repository.GetLeagueConfigStats(ctx)
	if err != nil {
		return nil, err
	}
	if configs == nil {
		return []models.LeagueConfigItem{}, nil
	}
	return configs, nil
}
