package advancedstats

import (
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func TestCombineMultiMatchup(t *testing.T) {
	got := combineMultiMatchup([]models.MultiMatchupItem{
		{RunsScored: 10, BallsFaced: 6, Dismissals: 1, Fours: 1},
		{RunsScored: 20, BallsFaced: 12, Sixes: 2},
	})

	if got.RunsScored != 30 {
		t.Fatalf("RunsScored = %d, want 30", got.RunsScored)
	}
	if got.BallsFaced != 18 {
		t.Fatalf("BallsFaced = %d, want 18", got.BallsFaced)
	}
	if got.StrikeRate != 166.67 {
		t.Fatalf("StrikeRate = %v, want 166.67", got.StrikeRate)
	}
	if got.EconomyRate != 10 {
		t.Fatalf("EconomyRate = %v, want 10", got.EconomyRate)
	}
}

func TestTotalPagesWithInvalidLimit(t *testing.T) {
	if got := totalPages(10, 0); got != 0 {
		t.Fatalf("totalPages() = %d, want 0", got)
	}
}
