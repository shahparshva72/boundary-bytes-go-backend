package leagues

import (
	"errors"
	"strings"
	"testing"
)

func TestDefaultRegistryLoads(t *testing.T) {
	r, err := Parse(leaguesJSON, false)
	if err != nil {
		t.Fatalf("embedded leagues.json is invalid: %v", err)
	}

	for _, code := range []string{"IPL", "WPL", "BBL", "WBBL", "SA20"} {
		if !r.IsVisible(code) {
			t.Errorf("existing league %s must stay visible", code)
		}
	}
}

func TestResolveScope(t *testing.T) {
	r, err := Parse(leaguesJSON, false)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name                    string
		league, leagues, gender string
		want                    string
		wantErr                 bool
	}{
		{name: "no params leaves default to caller", want: ""},
		{name: "legacy single league", league: "IPL", want: "IPL"},
		{name: "multi league", leagues: "IPL, BBL,IPL", want: "IPL,BBL"},
		{name: "gender expands to visible leagues", gender: "female", want: "WPL,WBBL"},
		{name: "gender male", gender: "Male", want: "IPL,BBL,SA20"},
		{name: "league matching gender", league: "WPL", gender: "female", want: "WPL"},
		{name: "league of other gender", league: "IPL", gender: "female", wantErr: true},
		{name: "hidden league rejected", league: "PSL", wantErr: true},
		{name: "unknown league", league: "XYZ", wantErr: true},
		{name: "lowercase code rejected like before", league: "ipl", wantErr: true},
		{name: "both league and leagues", league: "IPL", leagues: "BBL", wantErr: true},
		{name: "bad gender", gender: "mixed", wantErr: true},
		{name: "empty list", leagues: " , ", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := r.ResolveScope(tt.league, tt.leagues, tt.gender)
			if tt.wantErr {
				var scopeErr *ScopeError
				if !errors.As(err, &scopeErr) {
					t.Fatalf("want ScopeError, got %v (scope %q)", err, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Errorf("ResolveScope = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShowHiddenServesEveryLeague(t *testing.T) {
	r, err := Parse(leaguesJSON, true)
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.ResolveScope("PSL", "", "")
	if err != nil || got != "PSL" {
		t.Fatalf("ResolveScope(PSL) with hidden leagues shown = %q, %v", got, err)
	}
	if n := len(r.VisibleCodes("")); n != 28 {
		t.Errorf("visible leagues = %d, want 28", n)
	}
}

func TestTeamNameSQLMatchesLegacyMapping(t *testing.T) {
	r, err := Parse(leaguesJSON, false)
	if err != nil {
		t.Fatal(err)
	}

	// The four mappings the API hardcoded before leagues.json must survive.
	sql := r.TeamNameSQL("d.batting_team")
	for _, want := range []string{
		"WHEN d.batting_team = 'Royal Challengers Bengaluru' THEN 'Royal Challengers Bangalore'",
		"WHEN d.batting_team = 'Delhi Daredevils' THEN 'Delhi Capitals'",
		"WHEN d.batting_team = 'Kings XI Punjab' THEN 'Punjab Kings'",
		"WHEN d.batting_team = 'Rising Pune Supergiants' THEN 'Rising Pune Supergiant'",
		"ELSE d.batting_team",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("TeamNameSQL missing %q\n%s", want, sql)
		}
	}
}

func TestConflictingTeamRenameRejected(t *testing.T) {
	data := []byte(`{"leagues":[
		{"code":"A","name":"A","gender":"male","teamRenames":{"Old":"New"}},
		{"code":"B","name":"B","gender":"male","teamRenames":{"Old":"Other"}}
	]}`)
	if _, err := Parse(data, false); err == nil {
		t.Fatal("a variant mapped to two different teams must be rejected")
	}
}

func TestOverLength(t *testing.T) {
	r, err := Parse(leaguesJSON, true)
	if err != nil {
		t.Fatal(err)
	}
	tests := map[string]int{"IPL": 6, "HND": 5, "HND,WHND": 5, "IPL,HND": 6, "": 6, "NOPE": 6}
	for scope, want := range tests {
		if got := r.OverLength(scope); got != want {
			t.Errorf("OverLength(%q) = %d, want %d", scope, got, want)
		}
	}
}
