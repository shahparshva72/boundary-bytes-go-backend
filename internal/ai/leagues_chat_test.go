package ai

import (
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
)

func TestMain(m *testing.M) {
	// Every league visible, as on staging, so detection covers them all.
	os.Setenv("LEAGUES_SHOW_HIDDEN", "true")
	os.Exit(m.Run())
}

func TestDetectLeagues(t *testing.T) {
	r := leagues.Default()
	tests := map[string][]string{
		"Top run scorers in the PSL":                            {"PSL"},
		"Most wickets in the Women's Super Smash":               {"WSSM"},
		"Most wickets in the womens super smash":                {"WSSM"},
		"Most sixes in the Super Smash":                         {"SSM"},
		"top run scorers in WBBL":                               {"WBBL"},
		"Most wickets in the Big Bash":                          {"BBL"},
		"Top run scorers in The Hundred men's competition":      {"HND"},
		"Most wickets in the Women's Hundred":                   {"WHND"},
		"Most sixes in the T20 Blast":                           {"NTB"},
		"Compare IPL and PSL strike rates":                      {"IPL", "PSL"},
		"Who has scored the most runs in T20 leagues?":          nil,
		"How many runs has Kieron Pollard scored in his career": nil,
	}
	for question, want := range tests {
		got := DetectLeagues(r, question)
		slices.Sort(got)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("DetectLeagues(%q) = %v, want %v", question, got, want)
		}
	}
}

func TestStripLeagueNames(t *testing.T) {
	r := leagues.Default()
	got := StripLeagueNames(r, "How many runs has Babar Azam scored in the PSL?")
	if strings.Contains(got, "psl") || !strings.Contains(got, "babar azam") {
		t.Errorf("StripLeagueNames = %q", got)
	}
}

func TestDefaultLeagues(t *testing.T) {
	r := leagues.Default()
	if got := DefaultLeagues(r, "Most wickets in women's T20 leagues"); !slices.Contains(got, "WPL") || slices.Contains(got, "IPL") {
		t.Errorf("women's question should default to women's leagues, got %v", got)
	}
	if got := DefaultLeagues(r, "Most sixes across all T20 leagues"); !slices.Contains(got, "IPL") || slices.Contains(got, "WPL") {
		t.Errorf("default should be men's leagues, got %v", got)
	}
}

func TestSeasonIndexLabelFor(t *testing.T) {
	seasons := SeasonIndex{
		"BBL": {{"2011/12", 2012}, {"2012/13", 2012}, {"2023/24", 2024}},
		"IPL": {{"2009/10", 2010}},
	}
	tests := []struct {
		code  string
		year  int
		want  string
		found bool
	}{
		{"BBL", 2024, "2023/24", true},
		{"BBL", 2012, "2011/12", true}, // two seasons in 2012: the one ending in it
		{"IPL", 2010, "2009/10", true},
		{"IPL", 2016, "", false},
	}
	for _, tt := range tests {
		got, ok := seasons.LabelFor(tt.code, tt.year)
		if got != tt.want || ok != tt.found {
			t.Errorf("LabelFor(%s, %d) = %q, %v; want %q, %v", tt.code, tt.year, got, ok, tt.want, tt.found)
		}
	}
}

func TestBuildMasterPrompt(t *testing.T) {
	prompt := BuildMasterPrompt(leagues.Default(), SeasonIndex{"BBL": {{"2023/24", 2024}}})

	if strings.Contains(prompt, "{{") {
		t.Fatal("prompt has unfilled template sections")
	}
	if strings.Contains(strings.ToLower(prompt), "default to ipl") || strings.Contains(prompt, "for IPL data") {
		t.Error("prompt still defaults to IPL")
	}
	for _, want := range []string{
		"- PSL (men): Pakistan Super League",
		"- WHND (women): The Hundred Women's Competition",
		"BBL: 2023/24 (2024)",
		"('Kings XI Punjab','Punjab Kings'),",
		"COALESCE(mi.balls_per_over, 6) = 5 THEN 5 ELSE 6",
		"COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)::DECIMAL / 6",
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("prompt missing %q", want)
		}
	}
	if strings.Contains(prompt, "BETWEEN 6 AND 14") || strings.Contains(prompt, "NULLIF(COUNT(*)::DECIMAL / 6") {
		t.Error("prompt still has fixed phase ranges or the all-deliveries economy")
	}
}
