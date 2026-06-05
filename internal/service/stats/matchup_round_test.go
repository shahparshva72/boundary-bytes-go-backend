package stats

import (
	"math/rand"
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func TestBuildMatchupRoundFromItemsMostDismissals(t *testing.T) {
	items := []models.MultiMatchupItem{
		{Opponent: "JJ Bumrah", RunsScored: 159, BallsFaced: 108, Dismissals: 5, StrikeRate: 147.22},
		{Opponent: "Y Chahal", RunsScored: 45, BallsFaced: 36, Dismissals: 2, StrikeRate: 125.0},
		{Opponent: "A Zampa", RunsScored: 33, BallsFaced: 16, Dismissals: 0, StrikeRate: 206.25},
		{Opponent: "H M Pandya", RunsScored: 80, BallsFaced: 50, Dismissals: 1, StrikeRate: 160.0},
	}

	round := buildMatchupRoundFromItems("V Kohli", items, rand.New(rand.NewSource(1)))
	if round == nil {
		t.Fatal("expected matchup round")
	}
	if round.CorrectOpponent != "JJ Bumrah" {
		t.Fatalf("CorrectOpponent = %q, want JJ Bumrah", round.CorrectOpponent)
	}
	if len(round.Options) != 3 {
		t.Fatalf("len(Options) = %d, want 3", len(round.Options))
	}
}

func TestBuildMatchupRoundFromItemsRejectsEmptyData(t *testing.T) {
	items := []models.MultiMatchupItem{
		{Opponent: "A", RunsScored: 0, BallsFaced: 0, Dismissals: 0, StrikeRate: 0},
	}

	if round := buildMatchupRoundFromItems("V Kohli", items, rand.New(rand.NewSource(1))); round != nil {
		t.Fatal("expected nil round for empty data")
	}
}
