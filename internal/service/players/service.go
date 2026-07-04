package players

import (
	"context"
	"errors"
	"sort"
	"sync"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	statsservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/stats"
	"golang.org/x/sync/errgroup"
)

type Repository interface {
	GetAllLeagues(ctx context.Context) ([]string, error)
	GetBattersByLeague(ctx context.Context, league string) ([]string, error)
	GetBowlersByLeague(ctx context.Context, league string) ([]string, error)
	GetPlayerBySlug(ctx context.Context, slug string) (string, []string, error)
	ListPlayerSlugs(ctx context.Context, limit, offset int) ([]models.PlayerSlugEntry, int, error)
	GetPlayerStyleByName(ctx context.Context, playerName string) (*models.PlayerProfileBio, error)
}

type StatsService interface {
	GetPlayerCompare(ctx context.Context, league string, players []string, seasons []string, team *string, statType string) (*statsservice.PlayerCompareResult, error)
}

type Service struct {
	repository Repository
	stats      StatsService
}

type ListResult struct {
	Players []string
	Leagues []string
}

type PlayerProfileResult struct {
	Slug        string
	Name        string
	Bio         *models.PlayerProfileBio
	LeagueStats []models.PlayerProfileLeagueStats
	Leagues     []string
}

var ErrNotFound = errors.New("player not found")

func New(repository Repository, stats StatsService) *Service {
	return &Service{repository: repository, stats: stats}
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

func (s *Service) GetPlayerProfile(ctx context.Context, slug string) (*PlayerProfileResult, error) {
	playerName, leagues, err := s.repository.GetPlayerBySlug(ctx, slug)
	if err != nil {
		return nil, err
	}
	if playerName == "" {
		return nil, ErrNotFound
	}

	bio, err := s.repository.GetPlayerStyleByName(ctx, playerName)
	if err != nil {
		return nil, err
	}

	leagueStatsMu := make([]models.PlayerProfileLeagueStats, 0, len(leagues))
	var eg errgroup.Group
	var mu sync.Mutex

	for _, league := range leagues {
		league := league
		eg.Go(func() error {
			compareResult, err := s.stats.GetPlayerCompare(ctx, league, []string{playerName}, nil, nil, "both")
			if err != nil {
				return err
			}
			if len(compareResult.Players) == 0 {
				return nil
			}
			player := compareResult.Players[0]
			if player.Batting == nil && player.Bowling == nil {
				return nil
			}
			mu.Lock()
			defer mu.Unlock()
			leagueStatsMu = append(leagueStatsMu, models.PlayerProfileLeagueStats{
				League:  league,
				Batting: player.Batting,
				Bowling: player.Bowling,
			})
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}

	// Preserve deterministic league order.
	leagueOrder := make(map[string]int, len(leagues))
	for i, league := range leagues {
		leagueOrder[league] = i
	}
	sortLeagueStats(leagueStatsMu, leagueOrder)

	return &PlayerProfileResult{
		Slug:        slug,
		Name:        playerName,
		Bio:         bio,
		LeagueStats: leagueStatsMu,
		Leagues:     s.availableLeagues(ctx),
	}, nil
}

func (s *Service) ListPlayerSlugs(ctx context.Context, page, limit int) ([]models.PlayerSlugEntry, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 50
	}
	if limit > 1000 {
		limit = 1000
	}
	offset := (page - 1) * limit
	return s.repository.ListPlayerSlugs(ctx, limit, offset)
}

func (s *Service) availableLeagues(ctx context.Context) []string {
	leagues, err := s.repository.GetAllLeagues(ctx)
	if err != nil || leagues == nil {
		return []string{}
	}
	return leagues
}

func sortLeagueStats(stats []models.PlayerProfileLeagueStats, order map[string]int) {
	sort.SliceStable(stats, func(i, j int) bool {
		return order[stats[i].League] < order[stats[j].League]
	})
}
