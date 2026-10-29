package ai

import (
	"fmt"
	"sort"
	"strings"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
)

// BuildMasterPrompt fills the system prompt with the visible leagues, their
// season labels and team renames. seasons may be nil (no split-year rules).
func BuildMasterPrompt(registry *leagues.Registry, seasons SeasonIndex) string {
	return strings.NewReplacer(
		"{{LEAGUES_AND_SEASONS}}", leaguesAndSeasonsSection(registry, seasons),
		"{{TEAM_MAP}}", teamMapSection(registry),
		"{{OVERS_AND_PHASES}}", oversAndPhasesSection(),
	).Replace(masterPromptTemplate)
}

func leaguesAndSeasonsSection(registry *leagues.Registry, seasons SeasonIndex) string {
	var b strings.Builder
	men, women := registry.VisibleCodes("male"), registry.VisibleCodes("female")

	b.WriteString("LEAGUES (m.league code: names used for it):\n")
	for _, l := range registry.Visible() {
		names := []string{l.Name}
		for _, n := range append(append([]string{}, l.Events...), chatAliases[l.Code]...) {
			if !containsFold(names, n) {
				names = append(names, n)
			}
		}
		gender := "men"
		if l.Gender == "female" {
			gender = "women"
		}
		fmt.Fprintf(&b, "- %s (%s): %s\n", l.Code, gender, strings.Join(names, "; "))
	}

	fmt.Fprintf(&b, `
LEAGUE FILTERS:
- A league named in the question: filter on its code, e.g. m.league = 'PSL'; several leagues: m.league IN (...).
- Men's and women's versions are separate leagues: "The Hundred" alone means HND, "Women's Hundred" means WHND; likewise Super Smash/Women's Super Smash, T20 Blast/Women's T20 Blast, CPL/Women's CPL, Big Bash/WBBL.
- No league named and a specific player is asked about: do not filter by league (all of the player's T20 league matches).
- No league named otherwise (leaderboards, records, team questions): men's leagues only, %s,
  unless the question says women/women's/female or names a women's team, then women's leagues only, %s.
- Team names repeat across leagues (Mumbai Indians play in the IPL and WPL, Perth Scorchers in the BBL and WBBL): a team named without a league follows the rule above.
- Matches are identified by league codes only; never filter on team names to pick a league.
`, sqlList("m.league", men), sqlList("m.league", women))

	b.WriteString(`
SEASONS:
- A year on its own ("in 2025", "IPL 2023") means matches played that calendar year: m.start_date >= 'YYYY-01-01' AND m.start_date <= 'YYYY-12-31'.
- An explicit two-year season ("2023-24", "2023/24 season") means m.season = '2023/24'.
`)
	if lines := seasons.PromptLines(); len(lines) > 0 {
		b.WriteString(`- Some seasons span two calendar years and are labelled 'YYYY/YY' in m.season. For these leagues, "<league> <year>" means the season mostly played in that year (shown in brackets); use m.season = '<label>'. If two seasons share a year, use the one ending in it.
`)
		b.WriteString(strings.Join(lines, "\n"))
		b.WriteString("\n")
	}

	return b.String()
}

func teamMapSection(registry *leagues.Registry) string {
	renames := registry.TeamRenames()
	variants := make([]string, 0, len(renames))
	for v := range renames {
		variants = append(variants, v)
	}
	sort.Strings(variants)

	lines := make([]string, 0, len(variants))
	for _, v := range variants {
		lines = append(lines, fmt.Sprintf("(%s,%s),", sqlQuote(v), sqlQuote(renames[v])))
	}
	return strings.Join(lines, "\n")
}

func oversAndPhasesSection() string {
	return `OVERS AND PHASES:
- d.ball is '<over>.<ball>' with a 0-based over: over_number = CAST(SPLIT_PART(d.ball, '.', 1) AS INTEGER), 0 to 19.
- The Hundred (HND, WHND) uses 5-ball sets numbered the same way (wpl_match_info.balls_per_over = 5); every other league uses 6-ball overs.
- Phases cover both formats. Join LEFT JOIN wpl_match_info mi ON mi.match_id = d.match_id and use exactly:
  CASE WHEN CAST(SPLIT_PART(d.ball, '.', 1) AS INTEGER) < (CASE WHEN COALESCE(mi.balls_per_over, 6) = 5 THEN 5 ELSE 6 END) THEN 'powerplay' WHEN CAST(SPLIT_PART(d.ball, '.', 1) AS INTEGER) >= 15 THEN 'death' ELSE 'middle' END
  Never use fixed over ranges for powerplay, middle or death.
- Specific over numbers ("the 20th over", "overs 1 to 3") mean 6-ball overs: add COALESCE(mi.balls_per_over, 6) = 6, unless only The Hundred is asked about.
- Innings totals are 100 balls in The Hundred and 120 elsewhere: when ranking team totals or innings scores across leagues, add COALESCE(mi.balls_per_over, 6) = 6 unless the question is about The Hundred.`
}

func sqlList(column string, codes []string) string {
	quoted := make([]string, len(codes))
	for i, c := range codes {
		quoted[i] = sqlQuote(c)
	}
	return fmt.Sprintf("%s IN (%s)", column, strings.Join(quoted, ", "))
}

func sqlQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
