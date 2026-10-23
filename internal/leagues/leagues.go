// Package leagues is the API's view of the league registry.
//
// leagues.json is a copy of boundary-bytes-parser/internal/leagues/leagues.json,
// which is the source of truth: the parser decides which Cricsheet matches are
// loaded under which code. Keep the two files identical.
//
// A league is only served when it is visible. Setting LEAGUES_SHOW_HIDDEN=true
// serves every league (for local and staging databases during rollout).
package leagues

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"os"
	"sort"
	"strings"
	"sync"
)

//go:embed leagues.json
var leaguesJSON []byte

type League struct {
	Code        string            `json:"code"`
	Name        string            `json:"name"`
	Gender      string            `json:"gender"`
	Source      string            `json:"source"`
	Events      []string          `json:"events"`
	TeamRenames map[string]string `json:"teamRenames,omitempty"`
	// BallsPerOver is 6 for T20 leagues and 5 for The Hundred, whose overs
	// are 5-ball sets (Cricsheet's balls_per_over).
	BallsPerOver int  `json:"ballsPerOver,omitempty"`
	Visible      bool `json:"visible"`
}

type Registry struct {
	leagues     []League
	byCode      map[string]League
	teamRenames map[string]string
}

var (
	defaultOnce     sync.Once
	defaultRegistry *Registry
)

// Default returns the registry built from the embedded leagues.json. It
// panics if the embedded file is invalid, which TestDefaultRegistryLoads
// catches before deploy.
func Default() *Registry {
	defaultOnce.Do(func() {
		r, err := Parse(leaguesJSON, os.Getenv("LEAGUES_SHOW_HIDDEN") == "true")
		if err != nil {
			panic(err)
		}
		defaultRegistry = r
	})
	return defaultRegistry
}

// Parse builds a registry from leagues.json content. showHidden marks every
// league visible.
func Parse(data []byte, showHidden bool) (*Registry, error) {
	var file struct {
		Leagues []League `json:"leagues"`
	}
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, fmt.Errorf("parse leagues.json: %w", err)
	}

	r := &Registry{
		byCode:      make(map[string]League, len(file.Leagues)),
		teamRenames: make(map[string]string),
	}

	for _, l := range file.Leagues {
		if l.Code == "" || l.Name == "" {
			return nil, fmt.Errorf("league %q: code and name are required", l.Code)
		}
		if l.Gender != "male" && l.Gender != "female" {
			return nil, fmt.Errorf("league %s: gender must be male or female, got %q", l.Code, l.Gender)
		}
		if _, dup := r.byCode[l.Code]; dup {
			return nil, fmt.Errorf("duplicate league code %s", l.Code)
		}
		if showHidden {
			l.Visible = true
		}
		if l.BallsPerOver == 0 {
			l.BallsPerOver = 6
		}
		if l.BallsPerOver != 5 && l.BallsPerOver != 6 {
			return nil, fmt.Errorf("league %s: ballsPerOver must be 5 or 6, got %d", l.Code, l.BallsPerOver)
		}

		// Team names are merged with one SQL expression for every league, so
		// a variant must mean the same team wherever it appears.
		for variant, canonical := range l.TeamRenames {
			if existing, ok := r.teamRenames[variant]; ok && existing != canonical {
				return nil, fmt.Errorf("team rename %q maps to both %q and %q", variant, existing, canonical)
			}
			r.teamRenames[variant] = canonical
		}

		r.leagues = append(r.leagues, l)
		r.byCode[l.Code] = l
	}

	return r, nil
}

func (r *Registry) Get(code string) (League, bool) {
	l, ok := r.byCode[strings.ToUpper(strings.TrimSpace(code))]
	return l, ok
}

// Visible returns the visible leagues in config order.
func (r *Registry) Visible() []League {
	var visible []League
	for _, l := range r.leagues {
		if l.Visible {
			visible = append(visible, l)
		}
	}
	return visible
}

// VisibleCodes returns visible league codes for a gender ("" for both).
func (r *Registry) VisibleCodes(gender string) []string {
	var codes []string
	for _, l := range r.leagues {
		if l.Visible && (gender == "" || l.Gender == gender) {
			codes = append(codes, l.Code)
		}
	}
	return codes
}

// IsVisible reports whether code is a known, visible league.
func (r *Registry) IsVisible(code string) bool {
	l, ok := r.byCode[code]
	return ok && l.Visible
}

// OverLength returns the over length that over-numbered stats use for a
// scope: 5 when every league in it plays 5-ball overs (The Hundred), else 6.
// Queries indexed by over number then count only matches of that length, so
// "over 16" never mixes a T20 over with a Hundred set.
func (r *Registry) OverLength(scope string) int {
	codes := strings.Split(scope, ",")
	for _, code := range codes {
		if l, ok := r.byCode[strings.TrimSpace(code)]; !ok || l.BallsPerOver != 5 {
			return 6
		}
	}
	return 5
}

// ScopeError is a client error in a league/leagues/gender request.
type ScopeError struct{ msg string }

func (e *ScopeError) Error() string { return e.msg }

// ResolveScope turns request parameters into a league scope: a
// comma-separated list of visible league codes, which repository queries
// filter with m.league = ANY(string_to_array($n, ',')).
//
//   - league=IPL                 -> "IPL" (legacy single-league requests)
//   - leagues=IPL,PSL            -> "IPL,PSL"
//   - gender=female              -> every visible women's league
//   - leagues=... with gender=... -> the leagues must all be of that gender
//
// It returns "" when no parameter is set, so callers can apply their default.
func (r *Registry) ResolveScope(league, leaguesParam, gender string) (string, error) {
	league = strings.TrimSpace(league)
	leaguesParam = strings.TrimSpace(leaguesParam)
	gender = strings.ToLower(strings.TrimSpace(gender))

	if league != "" && leaguesParam != "" {
		return "", &ScopeError{"use either league or leagues, not both"}
	}
	if gender != "" && gender != "male" && gender != "female" {
		return "", &ScopeError{fmt.Sprintf("Invalid gender: %s. Valid genders are: male, female", gender)}
	}
	if league != "" {
		leaguesParam = league
	}

	if leaguesParam == "" {
		if gender == "" {
			return "", nil
		}
		codes := r.VisibleCodes(gender)
		if len(codes) == 0 {
			return "", &ScopeError{fmt.Sprintf("No leagues available for gender %s", gender)}
		}
		return strings.Join(codes, ","), nil
	}

	seen := make(map[string]bool)
	var codes []string
	for raw := range strings.SplitSeq(leaguesParam, ",") {
		code := strings.TrimSpace(raw)
		if code == "" || seen[code] {
			continue
		}
		if !r.IsVisible(code) {
			valid := r.VisibleCodes("")
			sort.Strings(valid)
			return "", &ScopeError{fmt.Sprintf("Invalid league: %s. Valid leagues are: %s", code, strings.Join(valid, ", "))}
		}
		if gender != "" && r.byCode[code].Gender != gender {
			return "", &ScopeError{fmt.Sprintf("League %s is not a %s league", code, gender)}
		}
		seen[code] = true
		codes = append(codes, code)
	}
	if len(codes) == 0 {
		return "", &ScopeError{"leagues must list at least one league code"}
	}

	return strings.Join(codes, ","), nil
}

// TeamRenames returns every renamed franchise variant -> merged name.
func (r *Registry) TeamRenames() map[string]string {
	renames := make(map[string]string, len(r.teamRenames))
	maps.Copy(renames, r.teamRenames)
	return renames
}

// TeamNameSQL returns a CASE expression that merges renamed franchises in
// column, e.g. TeamNameSQL("d.batting_team"). Variants are sorted so the
// generated SQL is stable.
func (r *Registry) TeamNameSQL(column string) string {
	if len(r.teamRenames) == 0 {
		return column
	}

	variants := make([]string, 0, len(r.teamRenames))
	for v := range r.teamRenames {
		variants = append(variants, v)
	}
	sort.Strings(variants)

	var b strings.Builder
	b.WriteString("\n\tCASE")
	for _, v := range variants {
		fmt.Fprintf(&b, "\n\t\tWHEN %s = %s THEN %s", column, quote(v), quote(r.teamRenames[v]))
	}
	fmt.Fprintf(&b, "\n\t\tELSE %s\n\tEND", column)
	return b.String()
}

func quote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}
