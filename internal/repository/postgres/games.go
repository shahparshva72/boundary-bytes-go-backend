package postgres

import (
	"context"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

const batterBowlersH2HQuery = `
	SELECT
		d.bowler AS opponent,
		COALESCE(SUM(d.runs_off_bat), 0)::int AS runs_scored,
		COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)::int AS balls_faced,
		COUNT(CASE WHEN d.player_dismissed = d.striker THEN 1 END)::int AS dismissals,
		CASE
			WHEN COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0) > 0
			THEN ROUND((COALESCE(SUM(d.runs_off_bat), 0)::numeric / COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)) * 100, 2)
			ELSE 0
		END AS strike_rate
	FROM wpl_delivery d
	JOIN wpl_match m ON d.match_id = m.match_id
	WHERE m.league = $1
		AND d.striker = $2
		AND d.innings <= 2
	GROUP BY d.bowler
	HAVING COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0) >= 3
	ORDER BY balls_faced DESC
	LIMIT 10;
`

func (s *service) GetEligibleMatchupBatters(ctx context.Context, league, seed string) ([]string, error) {
	query := `
		WITH h2h AS (
			SELECT
				d.striker AS batter,
				d.bowler AS bowler
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE m.league = $1 AND d.innings <= 2
			GROUP BY d.striker, d.bowler
			HAVING COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0) >= 3
		)
		SELECT batter
		FROM h2h
		GROUP BY batter
		HAVING COUNT(*) >= 5
		ORDER BY md5(batter || $2);
	`

	rows, err := s.db.QueryContext(ctx, query, league, seed)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var batters []string
	for rows.Next() {
		var batter string
		if err := rows.Scan(&batter); err != nil {
			return nil, err
		}
		batters = append(batters, batter)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return batters, nil
}

func (s *service) GetBatterBowlersH2H(ctx context.Context, league, batter string) ([]models.MultiMatchupItem, error) {
	rows, err := s.db.QueryContext(ctx, batterBowlersH2HQuery, league, batter)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MultiMatchupItem
	for rows.Next() {
		var item models.MultiMatchupItem
		if err := rows.Scan(
			&item.Opponent,
			&item.RunsScored,
			&item.BallsFaced,
			&item.Dismissals,
			&item.StrikeRate,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}
