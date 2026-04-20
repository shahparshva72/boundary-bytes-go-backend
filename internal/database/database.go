package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

type Service interface {
	Health() map[string]string
	Close() error
	GetDB() *sql.DB
	GetBattersByLeague(ctx context.Context, league string) ([]string, error)
	GetBowlersByLeague(ctx context.Context, league string) ([]string, error)
	GetAllLeagues(ctx context.Context) ([]string, error)
	GetSeasonsByLeague(ctx context.Context, league string) ([]string, error)
	GetLatestMatchDate(ctx context.Context, league string) (*time.Time, error)
	GetMatchList(ctx context.Context, league string) ([]models.MatchListItem, error)
	GetMatches(ctx context.Context, league string, season *string, page, limit int) ([]models.MatchCard, int, []string, error)
	GetTeamWins(ctx context.Context, league string) ([]models.TeamWinsItem, error)
	GetTeamAverages(ctx context.Context, league string) ([]models.TeamAverageItem, error)
	GetRunRateTrend(ctx context.Context, league string, team *string) ([]models.RunRateTrendItem, error)
	GetTeamRunRateProgression(ctx context.Context, league, team, season string) ([]models.TeamRunRateProgressionPoint, models.TeamRunRateProgressionMetadata, error)
	GetPlayerCompare(ctx context.Context, league string, players []string, seasons []string, team *string, statType string) ([]models.PlayerComparePlayer, error)
	GetBowlingWicketTypes(ctx context.Context, league string, page, limit int) ([]models.BowlingWicketTypesItem, int, error)
	GetMultiMatchup(ctx context.Context, league, player string, opponents []string, mode string) ([]models.MultiMatchupItem, error)
	GetPlayerProgression(ctx context.Context, league, player string, innings *int) ([]models.PlayerProgressionPoint, models.PlayerProgressionMetadata, error)
	GetAdvancedStats(ctx context.Context, league, playerType string, player string, overs []int) (interface{}, int, error)
	GetFallOfWickets(ctx context.Context, league string, matchID int) (*models.FallOfWicketsResponse, error)
	GetMatchupStats(ctx context.Context, league, batter, bowler string) (*models.MatchupData, error)
	GetLeadingWicketTakers(ctx context.Context, league string, page, limit int) ([]models.WicketTaker, int, error)
	GetLeadingRunScorers(ctx context.Context, league string, page, limit int) ([]models.RunScorer, int, error)
}

type service struct {
	db *sql.DB
}

func New(connectionString string) (Service, error) {
	db, err := sql.Open("pgx", connectionString)
	if err != nil {
		return nil, err
	}

	// Set connection pool setgo rtings
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(25)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Verify connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &service{db: db}, nil
}

func (s *service) GetDB() *sql.DB {
	return s.db
}

func (s *service) Close() error {
	return s.db.Close()
}

func (s *service) Health() map[string]string {
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	stats := make(map[string]string)

	err := s.db.PingContext(ctx)
	if err != nil {
		stats["status"] = "down"
		stats["error"] = fmt.Sprintf("db down: %v", err)
		return stats
	}

	stats["status"] = "up"
	stats["message"] = "It's healthy"
	stats["open_connections"] = fmt.Sprintf("%d", s.db.Stats().OpenConnections)

	return stats
}
