package matches

import (
	"context"
	"testing"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

type fakeRepository struct {
	leagues    []string
	matchCards []models.MatchCard
	totalCount int
	seasons    []string
	latestDate *time.Time
}

func (r fakeRepository) GetAllLeagues(ctx context.Context) ([]string, error) {
	return r.leagues, nil
}

func (r fakeRepository) GetLatestMatchDate(ctx context.Context, league string) (*time.Time, error) {
	return r.latestDate, nil
}

func (r fakeRepository) GetMatchList(ctx context.Context, league string) ([]models.MatchListItem, error) {
	return []models.MatchListItem{{ID: 1}}, nil
}

func (r fakeRepository) GetMatches(ctx context.Context, league string, season *string, page, limit int) ([]models.MatchCard, int, []string, error) {
	return r.matchCards, r.totalCount, r.seasons, nil
}

func (r fakeRepository) GetSeasonsByLeague(ctx context.Context, league string) ([]string, error) {
	return r.seasons, nil
}

func (r fakeRepository) GetTeamAverages(ctx context.Context, league string) ([]models.TeamAverageItem, error) {
	return []models.TeamAverageItem{{Team: "Mumbai Indians"}}, nil
}

func (r fakeRepository) GetTeamWins(ctx context.Context, league string) ([]models.TeamWinsItem, error) {
	return []models.TeamWinsItem{{Team: "Mumbai Indians"}}, nil
}

func TestGetMatchesCalculatesTotalPages(t *testing.T) {
	service := New(fakeRepository{
		leagues:    []string{"WPL"},
		totalCount: 11,
		seasons:    []string{"2023"},
	})

	got, err := service.GetMatches(context.Background(), "WPL", nil, 1, 5)
	if err != nil {
		t.Fatalf("GetMatches() error = %v", err)
	}
	if got.TotalPages != 3 {
		t.Fatalf("TotalPages = %d, want 3", got.TotalPages)
	}
	if len(got.Leagues) != 1 || got.Leagues[0] != "WPL" {
		t.Fatalf("Leagues = %#v", got.Leagues)
	}
}

func TestTotalPagesReturnsZeroForInvalidLimit(t *testing.T) {
	if got := totalPages(10, 0); got != 0 {
		t.Fatalf("totalPages() = %d, want 0", got)
	}
}

func TestAvailableLeaguesReturnsEmptySliceForNilResult(t *testing.T) {
	service := New(fakeRepository{})

	got, err := service.GetSeasons(context.Background(), "WPL")
	if err != nil {
		t.Fatalf("GetSeasons() error = %v", err)
	}
	if got.Leagues == nil {
		t.Fatal("expected empty leagues slice, got nil")
	}
}
