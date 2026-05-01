package texttosql

import "testing"

func TestValidateQuestionRejectsInvalidCharacters(t *testing.T) {
	message := validateQuestion("top scorers <script>")
	if message == "" {
		t.Fatal("expected invalid characters to be rejected")
	}
}

func TestSanitizeQuestionNormalizesWhitespace(t *testing.T) {
	got := sanitizeQuestion("  top   scorers\nin WPL  ")
	want := "top scorers in WPL"
	if got != want {
		t.Fatalf("sanitizeQuestion() = %q, want %q", got, want)
	}
}

func TestNormalizeTeamResultsAggregatesCanonicalTeamNames(t *testing.T) {
	rows := []map[string]interface{}{
		{"team": "Royal Challengers Bengaluru", "wins": int64(2)},
		{"team": "Royal Challengers Bangalore", "wins": int64(3)},
	}

	got := normalizeTeamResults(rows)
	if len(got) != 1 {
		t.Fatalf("expected one normalized team row, got %d", len(got))
	}
	if got[0]["team"] != "Royal Challengers Bangalore" {
		t.Fatalf("team = %v, want Royal Challengers Bangalore", got[0]["team"])
	}
	if got[0]["wins"] != float64(5) {
		t.Fatalf("wins = %v, want 5", got[0]["wins"])
	}
}
