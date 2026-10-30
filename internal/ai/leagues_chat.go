package ai

import (
	"regexp"
	"sort"
	"strings"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
)

// chatAliases are names people use for a league that aren't its code,
// Cricsheet name or event names. They only help match questions.
var chatAliases = map[string][]string{
	"BBL":   {"Big Bash"},
	"WBBL":  {"Women's Big Bash"},
	"SA20":  {"SA 20", "South Africa T20"},
	"PSL":   {"HBL PSL"},
	"CPL":   {"Caribbean Premier League"},
	"WCL":   {"WCPL"},
	"ILT20": {"ILT 20", "ILT-20"},
	"NTB":   {"Blast", "Vitality Blast"},
	"WTB":   {"Women's Blast", "Women's Vitality Blast"},
	"SMA":   {"SMAT", "Mushtaq Ali"},
	"SSM":   {"Super Smash"},
	"WSSM":  {"Women's Super Smash"},
	"CTC":   {"CSA T20", "Ram Slam"},
	"HND":   {"The Hundred", "Hundred", "Men's Hundred"},
	"WHND":  {"Women's Hundred"},
	"MSL":   {"Mzansi"},
	"IPT":   {"Inter-Provincial", "Interprovincial"},
	"ETPL":  {"European T20", "ETPL"},
	"MCT":   {"Major Clubs"},
	"WSL":   {"Kia Super League", "KSL", "WCSL"},
	"WTC":   {"Women's T20 Challenge"},
	"MLC":   {"Major League Cricket"},
	"NPL":   {"Nepal Premier League"},
	"LPL":   {"Lanka Premier League"},
}

var womenPattern = regexp.MustCompile(`\b(women|women's|womens|female|ladies|girls)\b`)

// MentionsWomen reports whether a question asks about women's cricket.
func MentionsWomen(question string) bool {
	return womenPattern.MatchString(normalizeQuestion(question))
}

func normalizeQuestion(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "’", "'")
	return strings.ReplaceAll(s, "womens", "women's")
}

type leagueTerm struct {
	code string
	term string
}

// leagueTerms lists every name of every visible league, longest first, so
// "Women's Super Smash" wins over "Super Smash".
func leagueTerms(registry *leagues.Registry) []leagueTerm {
	var terms []leagueTerm
	for _, l := range registry.Visible() {
		names := append([]string{l.Code, l.Name}, l.Events...)
		names = append(names, chatAliases[l.Code]...)
		for _, n := range names {
			terms = append(terms, leagueTerm{code: l.Code, term: normalizeQuestion(n)})
		}
	}
	sort.SliceStable(terms, func(i, j int) bool { return len(terms[i].term) > len(terms[j].term) })
	return terms
}

// DetectLeagues returns the codes of the visible leagues a question names.
// Longer names are matched first and their text is consumed, so "WBBL" or
// "Women's Big Bash" doesn't also count as BBL.
func DetectLeagues(registry *leagues.Registry, question string) []string {
	codes, _ := matchLeagues(registry, question)
	return codes
}

// StripLeagueNames returns the question (lowercased) with every league name
// blanked out, so the remaining words can be read as a player's name.
func StripLeagueNames(registry *leagues.Registry, question string) string {
	_, rest := matchLeagues(registry, question)
	return strings.TrimSpace(rest)
}

func matchLeagues(registry *leagues.Registry, question string) ([]string, string) {
	text := " " + normalizeQuestion(question) + " "
	seen := map[string]bool{}
	var codes []string

	for _, t := range leagueTerms(registry) {
		pattern := regexp.MustCompile(`(^|[^a-z0-9'])` + regexp.QuoteMeta(t.term) + `($|[^a-z0-9])`)
		for {
			loc := pattern.FindStringIndex(text)
			if loc == nil {
				break
			}
			// Blank out the match so shorter names inside it don't match again.
			text = text[:loc[0]+1] + strings.Repeat(" ", loc[1]-loc[0]-2) + text[loc[1]-1:]
			if !seen[t.code] {
				seen[t.code] = true
				codes = append(codes, t.code)
			}
		}
	}
	return codes, text
}

// DefaultLeagues is the scope for a question that names no league: the
// visible women's leagues when it mentions women, else the men's leagues.
func DefaultLeagues(registry *leagues.Registry, question string) []string {
	if MentionsWomen(question) {
		return registry.VisibleCodes("female")
	}
	return registry.VisibleCodes("male")
}

// LeagueFilterSQL renders a league condition on m.league.
func LeagueFilterSQL(codes []string) string {
	if len(codes) == 1 {
		return "m.league = '" + codes[0] + "'"
	}
	return "m.league IN ('" + strings.Join(codes, "', '") + "')"
}
