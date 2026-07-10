package players

import (
	"context"
	"errors"
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
	statsservice "github.com/shahparshva72/boundary-bytes-go-backend/internal/service/stats"
)

type fakeRepository struct {
	leagues         []string
	batters         []string
	bowlers         []string
	battersErr      error
	slugName        string
	slugLeagues     []string
	slugErr         error
	style           *models.PlayerProfileBio
	styleErr        error
	listEntries     []models.PlayerSlugEntry
	listTotal       int
	listErr         error
}

func (r fakeRepository) GetAllLeagues(ctx context.Context) ([]string, error) {
	return r.leagues, nil
}

func (r fakeRepository) GetBattersByLeague(ctx context.Context, league string) ([]string, error) {
	return r.batters, r.battersErr
}

func (r fakeRepository) GetBowlersByLeague(ctx context.Context, league string) ([]string, error) {
	return r.bowlers, nil
}

func (r fakeRepository) GetPlayerBySlug(ctx context.Context, slug string) (string, []string, error) {
	return r.slugName, r.slugLeagues, r.slugErr
}

func (r fakeRepository) ListPlayerSlugs(ctx context.Context, limit, offset int) ([]models.PlayerSlugEntry, int, error) {
	return r.listEntries, r.listTotal, r.listErr
}

func (r fakeRepository) GetPlayerStyleByName(ctx context.Context, playerName string) (*models.PlayerProfileBio, error) {
	return r.style, r.styleErr
}

type fakeStatsService struct {
	result *statsservice.PlayerCompareResult
	err    error
}

func (s fakeStatsService) GetPlayerCompare(ctx context.Context, league string, players []string, seasons []string, team *string, statType string) (*statsservice.PlayerCompareResult, error) {
	return s.result, s.err
}

func TestGetBattersReturnsPlayersAndLeagues(t *testing.T) {
	service := New(fakeRepository{
		leagues: []string{"WPL", "IPL"},
		batters: []string{"Smriti Mandhana"},
	}, fakeStatsService{})

	got, err := service.GetBatters(context.Background(), "WPL")
	if err != nil {
		t.Fatalf("GetBatters() error = %v", err)
	}
	if len(got.Players) != 1 || got.Players[0] != "Smriti Mandhana" {
		t.Fatalf("players = %#v", got.Players)
	}
	if len(got.Leagues) != 2 {
		t.Fatalf("leagues = %#v", got.Leagues)
	}
}

func TestGetBattersReturnsRepositoryError(t *testing.T) {
	expected := errors.New("database down")
	service := New(fakeRepository{battersErr: expected}, fakeStatsService{})

	_, err := service.GetBatters(context.Background(), "WPL")
	if !errors.Is(err, expected) {
		t.Fatalf("GetBatters() error = %v, want %v", err, expected)
	}
}

func TestGetPlayerProfileReturnsNotFoundForUnknownSlug(t *testing.T) {
	service := New(fakeRepository{slugName: ""}, fakeStatsService{})

	_, err := service.GetPlayerProfile(context.Background(), "unknown-player")
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("GetPlayerProfile() error = %v, want ErrNotFound", err)
	}
}

func TestGetPlayerProfileReturnsSingleLeagueStats(t *testing.T) {
	batting := &models.PlayerCompareBatting{Runs: 1000, BallsFaced: 800, Innings: 25}
	repo := fakeRepository{
		leagues:     []string{"IPL"},
		slugName:    "Virat Kohli",
		slugLeagues: []string{"IPL"},
		style: &models.PlayerProfileBio{
			PlayingRole: strPtr("batter"),
		},
	}
	statsSvc := fakeStatsService{
		result: &statsservice.PlayerCompareResult{
			Players: []models.PlayerComparePlayer{
				{Name: "Virat Kohli", Batting: batting},
			},
			Leagues: []string{"IPL"},
		},
	}
	service := New(repo, statsSvc)

	profile, err := service.GetPlayerProfile(context.Background(), "virat-kohli")
	if err != nil {
		t.Fatalf("GetPlayerProfile() error = %v", err)
	}
	if profile.Name != "Virat Kohli" {
		t.Fatalf("name = %q, want %q", profile.Name, "Virat Kohli")
	}
	if len(profile.LeagueStats) != 1 {
		t.Fatalf("league stats count = %d, want 1", len(profile.LeagueStats))
	}
	if profile.LeagueStats[0].League != "IPL" {
		t.Fatalf("league = %q, want IPL", profile.LeagueStats[0].League)
	}
	if profile.LeagueStats[0].Batting.Runs != 1000 {
		t.Fatalf("runs = %d, want 1000", profile.LeagueStats[0].Batting.Runs)
	}
	if profile.Bio == nil || profile.Bio.PlayingRole == nil || *profile.Bio.PlayingRole != "batter" {
		t.Fatalf("bio = %#v", profile.Bio)
	}
}

func TestGetPlayerProfileReturnsMultiLeagueStats(t *testing.T) {
	repo := fakeRepository{
		leagues:     []string{"IPL", "WPL"},
		slugName:    "Player One",
		slugLeagues: []string{"IPL", "WPL"},
	}
	statsSvc := fakeStatsService{
		result: &statsservice.PlayerCompareResult{
			Players: []models.PlayerComparePlayer{
				{Name: "Player One", Batting: &models.PlayerCompareBatting{Runs: 500}},
			},
		},
	}
	service := New(repo, statsSvc)

	profile, err := service.GetPlayerProfile(context.Background(), "player-one")
	if err != nil {
		t.Fatalf("GetPlayerProfile() error = %v", err)
	}
	if len(profile.LeagueStats) != 2 {
		t.Fatalf("league stats count = %d, want 2", len(profile.LeagueStats))
	}
}

func TestGetPlayerProfileOmitsEmptyLeagues(t *testing.T) {
	repo := fakeRepository{
		leagues:     []string{"IPL"},
		slugName:    "Player One",
		slugLeagues: []string{"IPL"},
	}
	statsSvc := fakeStatsService{
		result: &statsservice.PlayerCompareResult{
			Players: []models.PlayerComparePlayer{
				{Name: "Player One"},
			},
		},
	}
	service := New(repo, statsSvc)

	profile, err := service.GetPlayerProfile(context.Background(), "player-one")
	if err != nil {
		t.Fatalf("GetPlayerProfile() error = %v", err)
	}
	if len(profile.LeagueStats) != 0 {
		t.Fatalf("league stats count = %d, want 0", len(profile.LeagueStats))
	}
}

func TestGetPlayerProfilePropagatesStatsError(t *testing.T) {
	repo := fakeRepository{
		leagues:     []string{"IPL"},
		slugName:    "Player One",
		slugLeagues: []string{"IPL"},
	}
	statsSvc := fakeStatsService{err: errors.New("stats service down")}
	service := New(repo, statsSvc)

	_, err := service.GetPlayerProfile(context.Background(), "player-one")
	if err == nil {
		t.Fatalf("GetPlayerProfile() error = nil, want error")
	}
}

func TestListPlayerSlugsReturnsEntriesAndTotal(t *testing.T) {
	repo := fakeRepository{
		listEntries: []models.PlayerSlugEntry{
			{Slug: "virat-kohli", PlayerName: "Virat Kohli"},
		},
		listTotal: 1,
	}
	service := New(repo, fakeStatsService{})

	entries, total, err := service.ListPlayerSlugs(context.Background(), 1, 50)
	if err != nil {
		t.Fatalf("ListPlayerSlugs() error = %v", err)
	}
	if total != 1 {
		t.Fatalf("total = %d, want 1", total)
	}
	if len(entries) != 1 || entries[0].Slug != "virat-kohli" {
		t.Fatalf("entries = %#v", entries)
	}
}

func strPtr(s string) *string {
	return &s
}
