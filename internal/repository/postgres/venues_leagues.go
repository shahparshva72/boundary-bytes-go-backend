package postgres

import (
	"context"
	"fmt"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

const matchTotalsCTE = `
	innings_totals AS (
		SELECT
			d.match_id,
			d.innings,
			MAX(d.batting_team) AS batting_team,
			SUM(d.runs_off_bat + d.extras) AS runs,
			COUNT(*) FILTER (WHERE d.wides = 0) AS balls
		FROM wpl_delivery d
		JOIN wpl_match m ON m.match_id = d.match_id
		WHERE m.league = ANY(string_to_array($1, ','))
			AND d.innings <= 2
			AND ($2::int = 0 OR EXTRACT(YEAR FROM m.start_date) >= $2::int)
		GROUP BY d.match_id, d.innings
	),
	match_totals AS (
		SELECT
			m.match_id,
			m.league,
			m.venue,
			mi.city,
			mi.winner,
			MAX(it.runs) FILTER (WHERE it.innings = 1) AS first_runs,
			MAX(it.runs) FILTER (WHERE it.innings = 2) AS second_runs,
			MAX(it.batting_team) FILTER (WHERE it.innings = 1) AS first_team,
			MAX(it.batting_team) FILTER (WHERE it.innings = 2) AS second_team,
			SUM(it.runs) AS runs,
			SUM(it.balls) AS balls
		FROM innings_totals it
		JOIN wpl_match m ON m.match_id = it.match_id
		LEFT JOIN wpl_match_info mi ON mi.match_id = it.match_id
		GROUP BY m.match_id, m.league, m.venue, mi.city, mi.winner
	)`

var venueSortColumns = map[string]string{
	"matches":         "matches DESC",
	"avgFirstInnings": "avg_first DESC",
	"runRate":         "run_rate DESC",
	"chaseWinPct":     "chase_win_pct DESC NULLS LAST",
}

func (s *service) GetVenueStats(ctx context.Context, league string, since, minMatches int, sort string, page, limit int) ([]models.VenueStatsItem, int, error) {
	orderBy, ok := venueSortColumns[sort]
	if !ok {
		orderBy = venueSortColumns["matches"]
	}

	query := fmt.Sprintf(`
		WITH %s
		SELECT
			venue,
			COALESCE(MAX(city), '') AS city,
			COUNT(*) AS matches,
			COALESCE(AVG(first_runs), 0) AS avg_first,
			COALESCE(AVG(second_runs), 0) AS avg_second,
			COALESCE(MAX(GREATEST(first_runs, second_runs)), 0) AS highest_total,
			COALESCE(MIN(first_runs), 0) AS lowest_total,
			CASE WHEN SUM(balls) > 0 THEN SUM(runs)::numeric / (SUM(balls)::numeric / 6) ELSE 0 END AS run_rate,
			COUNT(*) FILTER (WHERE winner IS NOT NULL AND second_team IS NOT NULL) AS decided,
			COUNT(*) FILTER (WHERE winner = first_team) AS bat_first_wins,
			COUNT(*) FILTER (WHERE winner = second_team) AS chase_wins,
			COUNT(*) FILTER (WHERE winner = second_team)::numeric * 100
				/ NULLIF(COUNT(*) FILTER (WHERE winner IS NOT NULL AND second_team IS NOT NULL), 0) AS chase_win_pct,
			COUNT(*) OVER () AS total_count
		FROM match_totals
		GROUP BY venue
		HAVING COUNT(*) >= $3
		ORDER BY %s, venue
		LIMIT $4 OFFSET $5`, matchTotalsCTE, orderBy)

	rows, err := s.db.QueryContext(ctx, query, league, since, minMatches, limit, (page-1)*limit)
	if err != nil {
		return nil, 0, fmt.Errorf("venue stats: %w", err)
	}
	defer rows.Close()

	items := []models.VenueStatsItem{}
	total := 0
	for rows.Next() {
		var (
			item        models.VenueStatsItem
			chaseWinPct *float64
		)
		if err := rows.Scan(&item.Venue, &item.City, &item.Matches, &item.AvgFirstInnings, &item.AvgSecondInnings,
			&item.HighestTotal, &item.LowestTotal, &item.RunRate, &item.Decided, &item.BatFirstWins, &item.ChaseWins,
			&chaseWinPct, &total); err != nil {
			return nil, 0, fmt.Errorf("venue stats: %w", err)
		}
		item.AvgFirstInnings = roundTo2(item.AvgFirstInnings)
		item.AvgSecondInnings = roundTo2(item.AvgSecondInnings)
		item.RunRate = roundTo2(item.RunRate)
		if chaseWinPct != nil {
			item.ChaseWinPct = roundTo2(*chaseWinPct)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("venue stats: %w", err)
	}

	return items, total, nil
}

func (s *service) GetLeagueComparison(ctx context.Context, league string, since int) ([]models.LeagueComparisonItem, error) {
	query := fmt.Sprintf(`
		WITH %s,
		match_agg AS (
			SELECT
				league,
				COUNT(*) AS matches,
				COALESCE(AVG(first_runs), 0) AS avg_first,
				COUNT(*) FILTER (WHERE winner IS NOT NULL AND second_team IS NOT NULL) AS decided,
				COUNT(*) FILTER (WHERE winner = second_team) AS chase_wins
			FROM match_totals
			GROUP BY league
		),
		balls AS (
			SELECT
				m.league,
				%s AS phase,
				d.runs_off_bat,
				d.extras,
				d.wides,
				d.noballs,
				d.player_dismissed,
				d.wicket_type
			FROM wpl_delivery d
			JOIN wpl_match m ON m.match_id = d.match_id
			%s
			WHERE m.league = ANY(string_to_array($1, ','))
				AND d.innings <= 2
				AND ($2::int = 0 OR EXTRACT(YEAR FROM m.start_date) >= $2::int)
		),
		ball_agg AS (
			SELECT
				league,
				SUM(runs_off_bat + extras) AS runs,
				COUNT(*) FILTER (WHERE wides = 0) AS balls,
				COALESCE(SUM(runs_off_bat + extras) FILTER (WHERE phase = 'powerplay'), 0) AS powerplay_runs,
				COUNT(*) FILTER (WHERE wides = 0 AND phase = 'powerplay') AS powerplay_balls,
				COALESCE(SUM(runs_off_bat + extras) FILTER (WHERE phase = 'middle'), 0) AS middle_runs,
				COUNT(*) FILTER (WHERE wides = 0 AND phase = 'middle') AS middle_balls,
				COALESCE(SUM(runs_off_bat + extras) FILTER (WHERE phase = 'death'), 0) AS death_runs,
				COUNT(*) FILTER (WHERE wides = 0 AND phase = 'death') AS death_balls,
				COUNT(*) FILTER (WHERE runs_off_bat IN (4, 6)) AS boundaries,
				COUNT(*) FILTER (WHERE runs_off_bat = 0 AND wides = 0 AND noballs = 0) AS dot_balls,
				COUNT(*) FILTER (WHERE wides = 0 AND noballs = 0) AS legal_balls,
				COUNT(*) FILTER (WHERE player_dismissed IS NOT NULL AND wicket_type IN %s) AS wickets
			FROM balls
			GROUP BY league
		)
		SELECT
			ma.league, ma.matches, ma.avg_first, ma.decided, ma.chase_wins,
			ba.runs, ba.balls, ba.powerplay_runs, ba.powerplay_balls, ba.middle_runs, ba.middle_balls,
			ba.death_runs, ba.death_balls, ba.boundaries, ba.dot_balls, ba.legal_balls, ba.wickets
		FROM match_agg ma
		JOIN ball_agg ba ON ba.league = ma.league
		ORDER BY ma.matches DESC`, matchTotalsCTE, phaseSQL(), matchInfoJoinSQL, battingDismissalTypesSQL)

	rows, err := s.db.QueryContext(ctx, query, league, since)
	if err != nil {
		return nil, fmt.Errorf("league comparison: %w", err)
	}
	defer rows.Close()

	runRate := func(runs, balls int) float64 {
		if balls == 0 {
			return 0
		}
		return roundTo2(float64(runs) / (float64(balls) / 6))
	}

	items := []models.LeagueComparisonItem{}
	for rows.Next() {
		var (
			item                                           models.LeagueComparisonItem
			chaseWins, runs, balls                         int
			powerplayRuns, powerplayBalls                  int
			middleRuns, middleBalls, deathRuns, deathBalls int
			boundaries, dotBalls, legalBalls, wickets      int
		)
		if err := rows.Scan(&item.League, &item.Matches, &item.AvgFirstInnings, &item.Decided, &chaseWins,
			&runs, &balls, &powerplayRuns, &powerplayBalls, &middleRuns, &middleBalls,
			&deathRuns, &deathBalls, &boundaries, &dotBalls, &legalBalls, &wickets); err != nil {
			return nil, fmt.Errorf("league comparison: %w", err)
		}
		if l, ok := leagues.Default().Get(item.League); ok {
			item.Name = l.Name
		}
		item.AvgFirstInnings = roundTo2(item.AvgFirstInnings)
		item.RunRate = runRate(runs, balls)
		item.PowerplayRunRate = runRate(powerplayRuns, powerplayBalls)
		item.MiddleRunRate = runRate(middleRuns, middleBalls)
		item.DeathRunRate = runRate(deathRuns, deathBalls)
		if balls > 0 {
			item.BoundaryPct = roundTo2(float64(boundaries) / float64(balls) * 100)
			item.DotBallPct = roundTo2(float64(dotBalls) / float64(balls) * 100)
		}
		if wickets > 0 {
			item.BallsPerWicket = roundTo2(float64(legalBalls) / float64(wickets))
		}
		if item.Decided > 0 {
			item.ChaseWinPct = roundTo2(float64(chaseWins) / float64(item.Decided) * 100)
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("league comparison: %w", err)
	}

	return items, nil
}
