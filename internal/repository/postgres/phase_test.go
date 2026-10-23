package postgres

import (
	"strings"
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func TestPhaseForOver(t *testing.T) {
	tests := []struct {
		over, ballsPerOver int
		want               string
	}{
		{1, 6, "powerplay"}, {6, 6, "powerplay"}, {7, 6, "middle"}, {15, 6, "middle"}, {16, 6, "death"}, {20, 6, "death"},
		// The Hundred: 25-ball powerplay is sets 1-5; death is the last 5 sets.
		{5, 5, "powerplay"}, {6, 5, "middle"}, {15, 5, "middle"}, {16, 5, "death"},
	}
	for _, tt := range tests {
		if got := phaseForOver(tt.over, tt.ballsPerOver); got != tt.want {
			t.Errorf("phaseForOver(%d, %d) = %s, want %s", tt.over, tt.ballsPerOver, got, tt.want)
		}
	}
}

func TestOversNotation(t *testing.T) {
	if got := oversNotation(22, 6); got != 3.4 {
		t.Errorf("22 balls of 6-ball overs = %v, want 3.4", got)
	}
	if got := oversNotation(22, 5); got != 4.2 {
		t.Errorf("22 balls of 5-ball sets = %v, want 4.2", got)
	}
	if got := oversNotation(22, 0); got != 0 {
		t.Errorf("mixed over lengths should give 0, got %v", got)
	}
}

func TestAdvancedBowlerEconomyUsesLegalBalls(t *testing.T) {
	// 21 dot balls, 1 ball for 22 runs (off the bat), 1 wide and 1 no-ball:
	// 22 legal balls, 24 runs conceded.
	deliveries := make([]deliveryRow, 0, 24)
	for range 21 {
		deliveries = append(deliveries, deliveryRow{ballsPerOver: 6})
	}
	deliveries = append(deliveries,
		deliveryRow{runsOffBat: 22, ballsPerOver: 6},
		deliveryRow{wides: 1, extras: 1, ballsPerOver: 6},
		deliveryRow{noballs: 1, extras: 1, ballsPerOver: 6},
	)

	got := calculateAdvancedBowlerStats(deliveries)
	if got.BallsBowled != 22 || got.RunsConceded != 24 {
		t.Fatalf("balls=%d runs=%d, want 22 legal balls and 24 runs", got.BallsBowled, got.RunsConceded)
	}
	// 24 runs / (22/6) overs = 6.545..., not 24 / 3.4 = 7.06.
	if got.EconomyRate != 6.55 {
		t.Errorf("economy = %v, want 6.55", got.EconomyRate)
	}
	if got.Overs != 3.4 {
		t.Errorf("overs = %v, want 3.4", got.Overs)
	}

	deliveries[0].ballsPerOver = 5
	if got := calculateAdvancedBowlerStats(deliveries); got.Overs != 0 {
		t.Errorf("mixed over lengths should report overs 0, got %v", got.Overs)
	}
}

func TestStatExplorerOverAndPhaseFilters(t *testing.T) {
	from, to, phase := 1, 6, "powerplay"
	builder := &statExplorerSQLBuilder{}
	where := buildStatExplorerWhereClause(builder, "batting", models.StatExplorerRunFilters{OverFrom: &from, OverTo: &to}, "IPL,HND")

	if !strings.Contains(where, "COALESCE(mi.balls_per_over, 6) = 6") {
		t.Errorf("mixed scope should count only 6-ball overs:\n%s", where)
	}
	if !strings.Contains(where, overIndexSQL+" + 1 >= ") || !strings.Contains(where, overIndexSQL+" + 1 <= ") {
		t.Errorf("over filters should be 1-based:\n%s", where)
	}

	builder = &statExplorerSQLBuilder{}
	where = buildStatExplorerWhereClause(builder, "batting", models.StatExplorerRunFilters{Phase: &phase}, "HND")
	if !strings.Contains(where, "THEN 'powerplay'") || strings.Contains(where, "BETWEEN 1 AND 6") {
		t.Errorf("phase should use the format-aware expression:\n%s", where)
	}
}
