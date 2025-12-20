package database

import (
	"context"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func (s *service) GetBattersByLeague(ctx context.Context, league string) ([]string, error) {
	query := `
		SELECT DISTINCT d.striker
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		WHERE m.league = $1
		ORDER BY d.striker;
	`

	rows, err := s.db.QueryContext(ctx, query, league)
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

func (s *service) GetBowlersByLeague(ctx context.Context, league string) ([]string, error) {
	query := `
		SELECT DISTINCT d.bowler
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		WHERE m.league = $1
		ORDER BY d.bowler;
	`

	rows, err := s.db.QueryContext(ctx, query, league)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var bowlers []string
	for rows.Next() {
		var bowler string
		if err := rows.Scan(&bowler); err != nil {
			return nil, err
		}
		bowlers = append(bowlers, bowler)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return bowlers, nil
}

func (s *service) GetAllLeagues(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT league FROM wpl_match ORDER BY league;`
	rows, err := s.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var leagues []string
	for rows.Next() {
		var league string
		if err := rows.Scan(&league); err != nil {
			return nil, err
		}
		leagues = append(leagues, league)
	}
	return leagues, nil
}

func (s *service) GetMatchupStats(ctx context.Context, league, batter, bowler string) (*models.MatchupData, error) {
	query := `
		SELECT
			COALESCE(SUM(d.runs_off_bat), 0)::int as "runsScored",
			COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)::int as "ballsFaced",
			COUNT(CASE WHEN d.player_dismissed = $2 THEN 1 END)::int as "dismissals",
			CASE
				WHEN COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0) > 0 THEN ROUND((COALESCE(SUM(d.runs_off_bat), 0)::numeric / COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)) * 100, 2)
				ELSE 0
			END as "strikeRate",
			CASE
				WHEN COUNT(CASE WHEN d.player_dismissed = $2 THEN 1 END) > 0
				THEN ROUND(COALESCE(SUM(d.runs_off_bat), 0)::numeric / COUNT(CASE WHEN d.player_dismissed = $2 THEN 1 END), 2)
				ELSE COALESCE(SUM(d.runs_off_bat), 0)::numeric
			END as "average"
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		WHERE d.striker = $2 AND d.bowler = $3 AND m.league = $1 AND d.innings <= 2
	`

	var stats models.MatchupData
	err := s.db.QueryRowContext(ctx, query, league, batter, bowler).Scan(
		&stats.RunsScored,
		&stats.BallsFaced,
		&stats.Dismissals,
		&stats.StrikeRate,
		&stats.Average,
	)
	if err != nil {
		return nil, err
	}

	return &stats, nil
}
