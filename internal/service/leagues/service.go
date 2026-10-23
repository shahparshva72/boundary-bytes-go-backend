package leagues

import (
	"context"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

type Repository interface {
	GetLeagueConfigStats(ctx context.Context) ([]models.LeagueConfigItem, error)
}

type Service struct {
	repository Repository
	registry   *leagues.Registry
}

func New(repository Repository, registry *leagues.Registry) *Service {
	return &Service{repository: repository, registry: registry}
}

// GetConfigs lists every visible league in leagues.json order, with its
// Cricsheet name and gender, and the stats of whatever is loaded for it.
// Leagues with no matches yet get zero stats; hidden leagues are left out.
func (s *Service) GetConfigs(ctx context.Context) ([]models.LeagueConfigItem, error) {
	stats, err := s.repository.GetLeagueConfigStats(ctx)
	if err != nil {
		return nil, err
	}

	statsByLeague := make(map[string]models.LeagueConfigStats, len(stats))
	for _, item := range stats {
		statsByLeague[item.League] = item.Stats
	}

	configs := []models.LeagueConfigItem{}
	for _, l := range s.registry.Visible() {
		leagueStats := statsByLeague[l.Code]
		if leagueStats.Seasons == nil {
			leagueStats.Seasons = []string{}
		}
		configs = append(configs, models.LeagueConfigItem{
			League: l.Code,
			Name:   l.Name,
			Gender: l.Gender,
			Stats:  leagueStats,
		})
	}

	return configs, nil
}
