package advancedstats

import (
	"context"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

type Repository interface {
	GetAdvancedStats(ctx context.Context, league, playerType string, player string, overs []int) (interface{}, int, error)
	GetAllLeagues(ctx context.Context) ([]string, error)
	GetBowlingWicketTypes(ctx context.Context, league string, page, limit int) ([]models.BowlingWicketTypesItem, int, error)
	GetFallOfWickets(ctx context.Context, league string, matchID int) (*models.FallOfWicketsResponse, error)
	GetMultiMatchup(ctx context.Context, league, player string, opponents []string, mode string) ([]models.MultiMatchupItem, error)
	GetPlayerProgression(ctx context.Context, league, player string, innings *int) ([]models.PlayerProgressionPoint, models.PlayerProgressionMetadata, error)
}

type Service struct {
	repository Repository
}

type BowlingWicketTypesResult struct {
	Items      []models.BowlingWicketTypesItem
	TotalCount int
	TotalPages int
	Leagues    []string
}

type MultiMatchupResult struct {
	Items    []models.MultiMatchupItem
	Combined models.MultiMatchupCombined
	Leagues  []string
}

type PlayerProgressionResult struct {
	Items    []models.PlayerProgressionPoint
	Metadata models.PlayerProgressionMetadata
}

type AdvancedStatsResult struct {
	Data          interface{}
	DeliveryCount int
	Leagues       []string
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

func (s *Service) GetBowlingWicketTypes(ctx context.Context, league string, page, limit int) (*BowlingWicketTypesResult, error) {
	items, totalCount, err := s.repository.GetBowlingWicketTypes(ctx, league, page, limit)
	if err != nil {
		return nil, err
	}
	return &BowlingWicketTypesResult{
		Items:      items,
		TotalCount: totalCount,
		TotalPages: totalPages(totalCount, limit),
		Leagues:    s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetMultiMatchup(ctx context.Context, league, player string, opponents []string, mode string) (*MultiMatchupResult, error) {
	items, err := s.repository.GetMultiMatchup(ctx, league, player, opponents, mode)
	if err != nil {
		return nil, err
	}
	return &MultiMatchupResult{
		Items:    items,
		Combined: combineMultiMatchup(items),
		Leagues:  s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetPlayerProgression(ctx context.Context, league, player string, innings *int) (*PlayerProgressionResult, error) {
	items, metadata, err := s.repository.GetPlayerProgression(ctx, league, player, innings)
	if err != nil {
		return nil, err
	}
	metadata.AvailableLeagues = s.availableLeagues(ctx)
	return &PlayerProgressionResult{Items: items, Metadata: metadata}, nil
}

func (s *Service) GetAdvancedStats(ctx context.Context, league, playerType, player string, overs []int) (*AdvancedStatsResult, error) {
	data, deliveryCount, err := s.repository.GetAdvancedStats(ctx, league, playerType, player, overs)
	if err != nil {
		return nil, err
	}
	return &AdvancedStatsResult{
		Data:          data,
		DeliveryCount: deliveryCount,
		Leagues:       s.availableLeagues(ctx),
	}, nil
}

func (s *Service) GetFallOfWickets(ctx context.Context, league string, matchID int) (*models.FallOfWicketsResponse, error) {
	data, err := s.repository.GetFallOfWickets(ctx, league, matchID)
	if err != nil || data == nil {
		return data, err
	}
	data.Metadata.AvailableLeagues = s.availableLeagues(ctx)
	return data, nil
}

func (s *Service) availableLeagues(ctx context.Context) []string {
	leagues, err := s.repository.GetAllLeagues(ctx)
	if err != nil || leagues == nil {
		return []string{}
	}
	return leagues
}

func combineMultiMatchup(items []models.MultiMatchupItem) models.MultiMatchupCombined {
	combined := models.MultiMatchupCombined{}
	for _, item := range items {
		combined.RunsScored += item.RunsScored
		combined.BallsFaced += item.BallsFaced
		combined.Dismissals += item.Dismissals
		combined.Fours += item.Fours
		combined.Sixes += item.Sixes
		combined.DotBalls += item.DotBalls
	}

	if combined.BallsFaced > 0 {
		combined.StrikeRate = float64(combined.RunsScored) / float64(combined.BallsFaced) * 100
		combined.EconomyRate = float64(combined.RunsScored) / (float64(combined.BallsFaced) / 6)
	}
	if combined.Dismissals > 0 {
		combined.Average = float64(combined.RunsScored) / float64(combined.Dismissals)
	} else {
		combined.Average = float64(combined.RunsScored)
	}

	combined.StrikeRate = roundToTwo(combined.StrikeRate)
	combined.EconomyRate = roundToTwo(combined.EconomyRate)
	combined.Average = roundToTwo(combined.Average)
	return combined
}

func roundToTwo(value float64) float64 {
	if value == 0 {
		return 0
	}
	return float64(int(value*100+0.5)) / 100
}

func totalPages(totalCount, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (totalCount + limit - 1) / limit
}
