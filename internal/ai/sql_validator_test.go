package ai

import "testing"

func TestMinimalValidateAndNormalizeAddsLimit(t *testing.T) {
	query, err := MinimalValidateAndNormalize("SELECT player_name FROM wpl_player")
	if err != nil {
		t.Fatalf("expected valid query, got error: %v", err)
	}
	if query != "SELECT player_name FROM wpl_player LIMIT 20" {
		t.Fatalf("unexpected normalized query: %s", query)
	}
}

func TestMinimalValidateAndNormalizeCapsLimit(t *testing.T) {
	query, err := MinimalValidateAndNormalize("SELECT player_name FROM wpl_player LIMIT 100")
	if err != nil {
		t.Fatalf("expected valid query, got error: %v", err)
	}
	if query != "SELECT player_name FROM wpl_player LIMIT 20" {
		t.Fatalf("unexpected normalized query: %s", query)
	}
}

func TestValidateSQLRejectsWrites(t *testing.T) {
	result := ValidateSQL("DELETE FROM wpl_player")
	if result.IsValid {
		t.Fatal("expected write query to be rejected")
	}
}

func TestValidateSQLRejectsUnknownTables(t *testing.T) {
	result := ValidateSQL("SELECT * FROM users LIMIT 20")
	if result.IsValid {
		t.Fatal("expected unknown table to be rejected")
	}
}

func TestValidateSQLAllowsCTEs(t *testing.T) {
	result := ValidateSQL(`
		WITH batter_runs AS (
			SELECT striker, SUM(runs_off_bat) AS runs
			FROM wpl_delivery
			GROUP BY striker
		)
		SELECT striker, runs FROM batter_runs LIMIT 20
	`)
	if !result.IsValid {
		t.Fatalf("expected CTE query to be valid, got: %v", result.Errors)
	}
}

func TestValidateSQLAllowsPlayerStyleJoins(t *testing.T) {
	result := ValidateSQL(`
		SELECT d.striker, SUM(d.runs_off_bat) AS runs
		FROM wpl_delivery d
		JOIN wpl_match m ON m.match_id = d.match_id
		JOIN wpl_person_registry pr ON pr.match_id = d.match_id AND pr.person_name = d.bowler
		JOIN player_style ps ON ps.identifier = pr.registry_id
		WHERE m.league = 'IPL'
			AND d.innings <= 2
			AND ps.bowling_type = 'spin'
		GROUP BY d.striker
		LIMIT 20
	`)
	if !result.IsValid {
		t.Fatalf("expected player_style query to be valid, got: %v", result.Errors)
	}
}

func TestMinimalValidateAndNormalizeAllowsPlayerStyleJoins(t *testing.T) {
	query, err := MinimalValidateAndNormalize(`
		SELECT d.striker
		FROM wpl_delivery d
		JOIN wpl_person_registry pr ON pr.match_id = d.match_id AND pr.person_name = d.bowler
		JOIN player_style ps ON ps.identifier = pr.registry_id
		WHERE ps.bowling_sub_type IN ('left-arm-orthodox', 'left-arm-wrist-spin')
	`)
	if err != nil {
		t.Fatalf("expected player_style query to normalize, got error: %v", err)
	}
	if query == "" {
		t.Fatal("expected normalized query")
	}
}
