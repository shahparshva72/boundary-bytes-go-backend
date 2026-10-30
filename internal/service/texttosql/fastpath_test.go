package texttosql

import (
	"os"
	"strings"
	"testing"

	aisql "github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
)

func TestMain(m *testing.M) {
	// Every league visible, as on staging.
	os.Setenv("LEAGUES_SHOW_HIDDEN", "true")
	os.Exit(m.Run())
}

func onlyQuery(t *testing.T, queries []string) string {
	t.Helper()
	if len(queries) == 0 {
		t.Fatal("expected a fast-path query, got none")
	}
	q := queries[len(queries)-1]
	if v := aisql.ValidateSQL(q); !v.IsValid {
		t.Fatalf("invalid SQL %v:\n%s", v.Errors, q)
	}
	return q
}

func TestFastPathLeagueScope(t *testing.T) {
	tests := []struct {
		name, intent, question string
		want, notWant          []string
	}{
		{"named league", "leading_run_scorers", "Top run scorers in the PSL", []string{"m.league = 'PSL'"}, nil},
		{"no league: men's leagues", "leading_run_scorers", "Who has scored the most runs in T20 leagues?",
			[]string{"m.league IN ('IPL', 'BBL'", "'HND'"}, []string{"'WPL'"}},
		{"women's question: women's leagues", "leading_wicket_takers", "Most wickets in women's T20 leagues",
			[]string{"'WPL'", "'WHND'"}, []string{"'IPL'"}},
		{"named player, no league: all leagues", "player_batting_stat", "How many runs has Kieron Pollard scored in his career?",
			nil, []string{"m.league"}},
		{"named player and league", "player_batting_stat", "How many runs has Babar Azam scored in the PSL?",
			[]string{"m.league = 'PSL'"}, nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := onlyQuery(t, buildFastPathQueries(tt.intent, "unknown", "all", tt.question, nil))
			for _, w := range tt.want {
				if !strings.Contains(q, w) {
					t.Errorf("missing %q in:\n%s", w, q)
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(q, w) {
					t.Errorf("unexpected %q in:\n%s", w, q)
				}
			}
		})
	}
}

func TestFastPathPlayerNameIgnoresLeagueWords(t *testing.T) {
	queries := buildFastPathQueries("player_batting_stat", "unknown", "all", "How many runs has Babar Azam scored in the PSL?", nil)
	if len(queries) != 2 || !strings.Contains(queries[0], "ILIKE '%Azam%'") && !strings.Contains(queries[0], "ILIKE '%azam%'") {
		t.Fatalf("lookup should search for Azam, got %v", queries)
	}
	if !strings.Contains(queries[0], "m.league = 'PSL'") {
		t.Errorf("lookup should prefer PSL players:\n%s", queries[0])
	}
}

func TestFastPathSeasons(t *testing.T) {
	seasons := aisql.SeasonIndex{"BBL": {{Label: "2023/24", Year: 2024}}}

	q := onlyQuery(t, buildFastPathQueries("leading_run_scorers", "unknown", "all", "Top run scorers in BBL 2024", seasons))
	if !strings.Contains(q, "m.season = '2023/24'") {
		t.Errorf("BBL 2024 should be the 2023/24 season:\n%s", q)
	}
	q = onlyQuery(t, buildFastPathQueries("leading_run_scorers", "unknown", "all", "Top run scorers in the BBL 2023-24 season", seasons))
	if !strings.Contains(q, "m.season = '2023/24'") {
		t.Errorf("explicit 2023-24 should be that season:\n%s", q)
	}
	q = onlyQuery(t, buildFastPathQueries("most_sixes", "unknown", "all", "Most sixes in IPL 2023", seasons))
	if !strings.Contains(q, "m.start_date >= '2023-01-01'") {
		t.Errorf("IPL 2023 should be the calendar year:\n%s", q)
	}
}

func TestFastPathPhaseAndEconomy(t *testing.T) {
	q := onlyQuery(t, buildFastPathQueries("leading_wicket_takers", "unknown", "death", "Most wickets in the death overs in The Hundred", nil))
	if !strings.Contains(q, "LEFT JOIN wpl_match_info mi") || !strings.Contains(q, "= 'death'") || strings.Contains(q, "BETWEEN 15 AND 19") {
		t.Errorf("death phase should use the format-aware expression:\n%s", q)
	}
	if !strings.Contains(q, "NULLIF(COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)::DECIMAL / 6, 0)") {
		t.Errorf("economy should use legal balls:\n%s", q)
	}
}

func TestFastPathPlayerLookupIsRecognised(t *testing.T) {
	// The executor only resolves RESOLVED_PLAYER_NAME when it recognises the
	// lookup query; an unrecognised lookup leaves the placeholder in place.
	for _, tc := range []struct{ intent, question string }{
		{"player_batting_stat", "How many runs did Virat Kohli score in IPL 2016?"},
		{"player_bowling_stat", "How many wickets has Sunil Narine taken in total?"},
		{"head_to_head", "Virat Kohli vs Jasprit Bumrah"},
	} {
		queries := buildFastPathQueries(tc.intent, "unknown", "all", tc.question, nil)
		if len(queries) < 2 {
			t.Fatalf("%s: expected lookup + main query, got %v", tc.question, queries)
		}
		if !aisql.HasPlayerNameResolution(queries) {
			t.Errorf("%s: lookup not recognised by the executor:\n%s", tc.question, queries[0])
		}
		for _, lookup := range queries[:len(queries)-1] {
			if v := aisql.ValidateSQL(lookup); !v.IsValid {
				t.Errorf("%s: invalid lookup %v:\n%s", tc.question, v.Errors, lookup)
			}
		}
	}
}

func TestFastPathTeamQuestionUsesClassifiedLeague(t *testing.T) {
	q := onlyQuery(t, buildFastPathQueries("team_record", "BBL", "all", "How many matches have Perth Scorchers won?", nil))
	if !strings.Contains(q, "m.league = 'BBL'") {
		t.Errorf("team question should use the classified league:\n%s", q)
	}
	// Player questions ignore the classifier's league guess.
	queries := buildFastPathQueries("player_batting_stat", "IPL", "all", "How many runs has Kieron Pollard scored in his career?", nil)
	if strings.Contains(queries[len(queries)-1], "m.league") {
		t.Errorf("player question without a named league should cover all leagues:\n%s", queries[len(queries)-1])
	}
}

func TestExtractPlayerNameIgnoresFillerWords(t *testing.T) {
	for question, want := range map[string]string{
		"How many runs has Kieron Pollard scored in his career?":   "Pollard",
		"How many wickets has Sunil Narine taken in total?":        "Narine",
		"Babar Azam runs across all T20 leagues":                   "Azam",
		"Most runs by Sophie Devine in women's leagues since 2020": "Devine",
	} {
		if _, surname := extractPlayerName(question); !strings.EqualFold(surname, want) {
			t.Errorf("extractPlayerName(%q) surname = %q, want %q", question, surname, want)
		}
	}
}
