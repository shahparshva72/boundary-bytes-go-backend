package players

import (
	"context"
	"errors"
	"testing"
)

type fakeRepository struct {
	leagues    []string
	batters    []string
	bowlers    []string
	battersErr error
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

func TestGetBattersReturnsPlayersAndLeagues(t *testing.T) {
	service := New(fakeRepository{
		leagues: []string{"WPL", "IPL"},
		batters: []string{"Smriti Mandhana"},
	})

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
	service := New(fakeRepository{battersErr: expected})

	_, err := service.GetBatters(context.Background(), "WPL")
	if !errors.Is(err, expected) {
		t.Fatalf("GetBatters() error = %v, want %v", err, expected)
	}
}
