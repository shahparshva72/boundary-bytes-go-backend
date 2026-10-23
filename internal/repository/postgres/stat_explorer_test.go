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
