package games

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/repository/postgres"
)

var (
	ErrDeviceIDRequired = errors.New("deviceId is required")
	ErrDeviceIDTooLong  = errors.New("deviceId is too long")
	ErrDateRequired     = errors.New("date is required")
	ErrInvalidDate      = errors.New("date must be YYYY-MM-DD")
	ErrInvalidScore     = errors.New("score must be non-negative")
	ErrLineupRequired   = errors.New("lineup is required")
	ErrLineupTooLarge   = errors.New("lineup cannot exceed 8 players")
	ErrAlreadySubmitted = errors.New("score already submitted for this date")
)

var datePattern = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type Repository interface {
	InsertDailyDraftScore(ctx context.Context, params models.SubmitDailyDraftScoreParams) error
	GetDailyDraftLeaderboard(ctx context.Context, league, date, deviceID string, topN int) (models.DailyDraftLeaderboardResponse, error)
}

type Service struct {
	repository Repository
}

func New(repository Repository) *Service {
	return &Service{repository: repository}
}

type SubmitParams struct {
	DeviceID     string
	League       string
	Date         string
	Score        float64
	OptimalScore float64
	Lineup       []string
}

func (s *Service) SubmitScore(ctx context.Context, params SubmitParams) error {
	deviceID := strings.TrimSpace(params.DeviceID)
	if deviceID == "" {
		return ErrDeviceIDRequired
	}
	if len(deviceID) > 64 {
		return ErrDeviceIDTooLong
	}

	date := strings.TrimSpace(params.Date)
	if date == "" {
		return ErrDateRequired
	}
	if !datePattern.MatchString(date) {
		return ErrInvalidDate
	}

	if params.Score < 0 || params.OptimalScore < 0 {
		return ErrInvalidScore
	}

	if len(params.Lineup) == 0 {
		return ErrLineupRequired
	}
	if len(params.Lineup) > 8 {
		return ErrLineupTooLarge
	}

	lineup := make([]string, 0, len(params.Lineup))
	for _, name := range params.Lineup {
		trimmed := strings.TrimSpace(name)
		if trimmed != "" {
			lineup = append(lineup, trimmed)
		}
	}
	if len(lineup) == 0 {
		return ErrLineupRequired
	}

	err := s.repository.InsertDailyDraftScore(ctx, models.SubmitDailyDraftScoreParams{
		DeviceID:     deviceID,
		League:       params.League,
		Date:         date,
		Score:        params.Score,
		OptimalScore: params.OptimalScore,
		Lineup:       lineup,
	})
	if err != nil {
		if errors.Is(err, postgres.ErrDailyDraftAlreadySubmitted) {
			return ErrAlreadySubmitted
		}
		return err
	}

	return nil
}

func (s *Service) Leaderboard(ctx context.Context, league, date, deviceID string) (models.DailyDraftLeaderboardResponse, error) {
	date = strings.TrimSpace(date)
	if date == "" {
		return models.DailyDraftLeaderboardResponse{}, ErrDateRequired
	}
	if !datePattern.MatchString(date) {
		return models.DailyDraftLeaderboardResponse{}, ErrInvalidDate
	}

	return s.repository.GetDailyDraftLeaderboard(ctx, league, date, strings.TrimSpace(deviceID), 10)
}
