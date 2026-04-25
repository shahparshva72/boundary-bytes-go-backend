package ai

import "testing"

func TestBuildExecutableQueriesResolvesPlayerName(t *testing.T) {
	queries := []string{
		"SELECT ps.name AS player_name FROM player_style ps WHERE ps.name ILIKE '%Mandhana%' OR ps.full_name ILIKE '%Mandhana%' ORDER BY CASE WHEN ps.full_name ILIKE 'Smriti Mandhana' THEN 1 ELSE 2 END LIMIT 1",
		"SELECT d.striker FROM wpl_delivery d WHERE d.striker = 'RESOLVED_PLAYER_NAME' LIMIT 20",
	}

	built, err := BuildExecutableQueries(queries, func(query string) ([]map[string]interface{}, error) {
		return []map[string]interface{}{
			{"player_name": "S Mandhana"},
		}, nil
	})
	if err != nil {
		t.Fatalf("expected build to succeed, got error: %v", err)
	}
	if len(built) != 1 {
		t.Fatalf("expected one executable query, got %d", len(built))
	}
	expected := "SELECT d.striker FROM wpl_delivery d WHERE d.striker = 'S Mandhana' LIMIT 20"
	if built[0] != expected {
		t.Fatalf("unexpected built query:\nwant: %s\ngot:  %s", expected, built[0])
	}
}

func TestBuildExecutableQueriesStillSupportsWPLPlayerLookup(t *testing.T) {
	queries := []string{
		"SELECT player_name FROM wpl_player WHERE player_name ILIKE '%Mandhana%' ORDER BY CASE WHEN player_name ILIKE 'S%Mandhana' THEN 1 ELSE 2 END LIMIT 1",
		"SELECT d.striker FROM wpl_delivery d WHERE d.striker = 'RESOLVED_PLAYER_NAME' LIMIT 20",
	}

	built, err := BuildExecutableQueries(queries, func(query string) ([]map[string]interface{}, error) {
		return []map[string]interface{}{{"player_name": "S Mandhana"}}, nil
	})
	if err != nil {
		t.Fatalf("expected build to succeed, got error: %v", err)
	}
	expected := "SELECT d.striker FROM wpl_delivery d WHERE d.striker = 'S Mandhana' LIMIT 20"
	if built[0] != expected {
		t.Fatalf("unexpected built query:\nwant: %s\ngot:  %s", expected, built[0])
	}
}

func TestApplyResolvedNamesEscapesQuotes(t *testing.T) {
	query := "SELECT * FROM wpl_delivery d WHERE d.bowler = 'RESOLVED_BOWLER_NAME' LIMIT 20"
	result := ApplyResolvedNames(query, ResolvedNames{BowlerName: "D'Angelo"})
	expected := "SELECT * FROM wpl_delivery d WHERE d.bowler = 'D''Angelo' LIMIT 20"
	if result != expected {
		t.Fatalf("unexpected query:\nwant: %s\ngot:  %s", expected, result)
	}
}
