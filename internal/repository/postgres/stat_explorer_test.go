package postgres

import (
	"strings"
	"testing"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func TestBuildMatchReportQueriesGroupsMatchInfo(t *testing.T) {
	request := models.StatExplorerRunRequest{
		ReportType: "match",
		Dimensions: []string{"team", "season", "innings"},
		Metrics:    []string{"runs", "wickets"},
		Pagination: models.StatExplorerRunPagination{Page: 1, PageSize: 20},
	}

	built, err := buildStatExplorerQueries(request, "IPL")
	if err != nil {
		t.Fatalf("buildStatExplorerQueries() error = %v", err)
	}

	// mi.city, mi.toss_winner, ... are selected, so wpl_match_info's key must
	// be grouped or Postgres rejects the query.
	for _, q := range []string{built.sql, built.countSQL} {
		if !strings.Contains(q, "GROUP BY m.match_id, mi.match_id") {
			t.Errorf("query does not group by mi.match_id:\n%s", q)
		}
	}
	for _, col := range []string{`d.batting_team AS "team"`, `d.innings AS "innings"`} {
		if !strings.Contains(built.sql, col) {
			t.Errorf("grouped dimension not selected: %s", col)
		}
	}
}

func TestBuildBattingQueriesApplyQualifiersToRowsAndCount(t *testing.T) {
	minBalls, minRuns, minWickets := 100, 200, 5
	request := models.StatExplorerRunRequest{
		ReportType: "batting",
		Dimensions: []string{"player"},
		Metrics:    []string{"boundaryPct", "ballsPerSix", "nonBoundaryStrikeRate"},
		Filters:    models.StatExplorerRunFilters{MinBalls: &minBalls, MinRuns: &minRuns, MinWickets: &minWickets},
		Pagination: models.StatExplorerRunPagination{Page: 1, PageSize: 20},
	}

	built, err := buildStatExplorerQueries(request, "IPL")
	if err != nil {
		t.Fatalf("buildStatExplorerQueries() error = %v", err)
	}

	// league, minRuns, minBalls; wickets do not qualify a batting report.
	if len(built.args) != 3 {
		t.Fatalf("args = %v, want 3 values", built.args)
	}
	having := "HAVING SUM(stats.runs) >= $2 AND SUM(stats.balls_faced) >= $3"
	for _, q := range []string{built.sql, built.countSQL} {
		if !strings.Contains(q, having) {
			t.Errorf("query is missing %q:\n%s", having, q)
		}
	}
	if !strings.Contains(built.sql, "ORDER BY boundarypct DESC NULLS LAST") {
		t.Errorf("default order does not put empty rates last:\n%s", built.sql)
	}
	for _, metric := range []string{"boundarypct", "ballspersix", "nonboundarystrikerate"} {
		if !built.metricKeys[metric] {
			t.Errorf("metric %s not selected", metric)
		}
		if strings.Contains(built.sql, "0 AS "+metric) {
			t.Errorf("metric %s has no SQL expression", metric)
		}
	}
}

func TestBuildBowlingQueriesQualifyOnBowlingTotals(t *testing.T) {
	minBalls, minWickets := 60, 5
	request := models.StatExplorerRunRequest{
		ReportType: "bowling",
		Dimensions: []string{"player"},
		Metrics:    []string{"dotBallPct", "boundaryPct"},
		Filters:    models.StatExplorerRunFilters{MinBalls: &minBalls, MinWickets: &minWickets},
		Pagination: models.StatExplorerRunPagination{Page: 1, PageSize: 20},
	}

	built, err := buildStatExplorerQueries(request, "IPL")
	if err != nil {
		t.Fatalf("buildStatExplorerQueries() error = %v", err)
	}

	having := "HAVING SUM(stats.balls_bowled) >= $2 AND SUM(stats.wickets) >= $3"
	for _, q := range []string{built.sql, built.countSQL} {
		if !strings.Contains(q, having) {
			t.Errorf("query is missing %q:\n%s", having, q)
		}
	}
	if !strings.Contains(built.sql, "SUM(stats.boundaries)") {
		t.Errorf("boundary %% does not use boundaries conceded:\n%s", built.sql)
	}
}

func TestBuildTeamReportQueriesSplitByBattingOrder(t *testing.T) {
	request := models.StatExplorerRunRequest{
		ReportType: "team",
		Dimensions: []string{"team"},
		Metrics:    []string{"matchesBattingFirst", "winPctBattingFirst", "matchesBattingSecond", "winPctBattingSecond"},
		Pagination: models.StatExplorerRunPagination{Page: 1, PageSize: 20},
	}

	built, err := buildStatExplorerQueries(request, "IPL")
	if err != nil {
		t.Fatalf("buildStatExplorerQueries() error = %v", err)
	}

	for _, want := range []string{"1 AS bat_order", "2 AS bat_order", `AS "winPctBattingFirst"`, `AS "winPctBattingSecond"`} {
		if !strings.Contains(built.sql, want) {
			t.Errorf("query is missing %q:\n%s", want, built.sql)
		}
	}
}
