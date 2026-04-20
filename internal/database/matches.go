package database

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func (s *service) GetSeasonsByLeague(ctx context.Context, league string) ([]string, error) {
	query := `
		SELECT DISTINCT season
		FROM wpl_match_info
		WHERE league = $1
		ORDER BY season DESC;
	`

	rows, err := s.db.QueryContext(ctx, query, league)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var seasons []string
	for rows.Next() {
		var season string
		if err := rows.Scan(&season); err != nil {
			return nil, err
		}
		seasons = append(seasons, season)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return seasons, nil
}

func (s *service) GetLatestMatchDate(ctx context.Context, league string) (*time.Time, error) {
	query := `
		SELECT MAX(start_date)
		FROM wpl_match
		WHERE league = $1;
	`

	var latestDate sql.NullTime
	if err := s.db.QueryRowContext(ctx, query, league).Scan(&latestDate); err != nil {
		return nil, err
	}
	if !latestDate.Valid {
		return nil, nil
	}

	value := latestDate.Time
	return &value, nil
}

func (s *service) GetMatchList(ctx context.Context, league string) ([]models.MatchListItem, error) {
	query := `
		WITH match_teams AS (
			SELECT
				m.match_id,
				m.league,
				m.season,
				m.start_date,
				m.venue,
				STRING_AGG(DISTINCT
					CASE
						WHEN d.batting_team = 'Royal Challengers Bengaluru' THEN 'Royal Challengers Bangalore'
						WHEN d.batting_team = 'Delhi Daredevils' THEN 'Delhi Capitals'
						WHEN d.batting_team = 'Kings XI Punjab' THEN 'Punjab Kings'
						WHEN d.batting_team = 'Rising Pune Supergiants' THEN 'Rising Pune Supergiant'
						ELSE d.batting_team
					END, ' vs ' ORDER BY
					CASE
						WHEN d.batting_team = 'Royal Challengers Bengaluru' THEN 'Royal Challengers Bangalore'
						WHEN d.batting_team = 'Delhi Daredevils' THEN 'Delhi Capitals'
						WHEN d.batting_team = 'Kings XI Punjab' THEN 'Punjab Kings'
						WHEN d.batting_team = 'Rising Pune Supergiants' THEN 'Rising Pune Supergiant'
						ELSE d.batting_team
					END
				) AS teams
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE m.league = $1
			GROUP BY m.match_id, m.league, m.season, m.start_date, m.venue
		)
		SELECT
			match_id,
			league,
			season,
			start_date,
			venue,
			teams
		FROM match_teams
		ORDER BY start_date DESC;
	`

	rows, err := s.db.QueryContext(ctx, query, league)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.MatchListItem
	for rows.Next() {
		var item models.MatchListItem
		var startDate sql.NullTime
		if err := rows.Scan(&item.ID, &item.League, &item.Season, &startDate, &item.Venue, &item.Teams); err != nil {
			return nil, err
		}
		if startDate.Valid {
			item.Date = startDate.Time.Format("2006-01-02")
		} else {
			item.Date = ""
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *service) GetMatches(
	ctx context.Context,
	league string,
	season *string,
	page int,
	limit int,
) ([]models.MatchCard, int, []string, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 1
	}

	offset := (page - 1) * limit

	var seasonClause string
	args := []interface{}{league}
	argIndex := 2
	if season != nil && *season != "" {
		seasonClause = fmt.Sprintf("AND season = $%d", argIndex)
		args = append(args, *season)
		argIndex++
	}
	limitIndex := argIndex
	args = append(args, limit)
	argIndex++
	offsetIndex := argIndex
	args = append(args, offset)

	query := fmt.Sprintf(`
		WITH paginated_matches AS (
			SELECT match_id, league, season, date, venue, winner, winner_runs, winner_wickets
			FROM wpl_match_info
			WHERE league = $1 %s
			ORDER BY date ASC
			LIMIT $%d OFFSET $%d
		),
		match_scores AS (
			SELECT
				d.match_id,
				MAX(CASE WHEN d.innings = 1 THEN d.batting_team END) as team1,
				MAX(CASE WHEN d.innings = 1 THEN d.bowling_team END) as team2,
				COALESCE(SUM(CASE WHEN d.innings = 1 THEN d.runs_off_bat + d.extras ELSE 0 END), 0) as innings1_score,
				COALESCE(SUM(CASE WHEN d.innings = 2 THEN d.runs_off_bat + d.extras ELSE 0 END), 0) as innings2_score,
				(COUNT(CASE WHEN d.innings = 1 AND d.player_dismissed IS NOT NULL THEN 1 END) +
				 COUNT(CASE WHEN d.innings = 1 AND d.other_player_dismissed IS NOT NULL THEN 1 END)) as innings1_wickets,
				(COUNT(CASE WHEN d.innings = 2 AND d.player_dismissed IS NOT NULL THEN 1 END) +
				 COUNT(CASE WHEN d.innings = 2 AND d.other_player_dismissed IS NOT NULL THEN 1 END)) as innings2_wickets
			FROM wpl_delivery d
			WHERE d.match_id IN (SELECT match_id FROM paginated_matches)
			GROUP BY d.match_id
		),
		total AS (
			SELECT COUNT(*) as cnt FROM wpl_match_info WHERE league = $1 %s
		)
		SELECT
			pm.match_id,
			pm.league,
			pm.season,
			pm.date as start_date,
			pm.venue,
			pm.winner,
			pm.winner_runs,
			pm.winner_wickets,
			ms.team1,
			ms.team2,
			COALESCE(ms.innings1_score, 0) as innings1_score,
			COALESCE(ms.innings2_score, 0) as innings2_score,
			COALESCE(ms.innings1_wickets, 0) as innings1_wickets,
			COALESCE(ms.innings2_wickets, 0) as innings2_wickets,
			(SELECT cnt FROM total) as total_count
		FROM paginated_matches pm
		LEFT JOIN match_scores ms ON pm.match_id = ms.match_id
		ORDER BY pm.date ASC;
	`, seasonClause, limitIndex, offsetIndex, seasonClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, nil, err
	}
	defer rows.Close()

	var matches []models.MatchCard
	totalCount := 0
	for rows.Next() {
		var (
			matchID         int
			matchLeague     string
			matchSeason     string
			startDate       sql.NullTime
			venue           string
			winner          sql.NullString
			winnerRuns      sql.NullInt64
			winnerWickets   sql.NullInt64
			team1           sql.NullString
			team2           sql.NullString
			innings1Score   int
			innings2Score   int
			innings1Wickets int
			innings2Wickets int
			totalCountValue int
		)

		if err := rows.Scan(
			&matchID,
			&matchLeague,
			&matchSeason,
			&startDate,
			&venue,
			&winner,
			&winnerRuns,
			&winnerWickets,
			&team1,
			&team2,
			&innings1Score,
			&innings2Score,
			&innings1Wickets,
			&innings2Wickets,
			&totalCountValue,
		); err != nil {
			return nil, 0, nil, err
		}

		if totalCount == 0 {
			totalCount = totalCountValue
		}

		result := formatMatchResult(
			winner,
			winnerRuns,
			winnerWickets,
			team1,
			team2,
			innings1Score,
			innings2Score,
			innings2Wickets,
		)

		match := models.MatchCard{
			ID:            matchID,
			League:        matchLeague,
			Season:        matchSeason,
			Venue:         venue,
			Team1:         team1.String,
			Team2:         team2.String,
			Innings1Score: fmt.Sprintf("%d/%d", innings1Score, innings1Wickets),
			Innings2Score: fmt.Sprintf("%d/%d", innings2Score, innings2Wickets),
			Result:        result,
		}
		if startDate.Valid {
			match.StartDate = startDate.Time
		}

		matches = append(matches, match)
	}

	if err := rows.Err(); err != nil {
		return nil, 0, nil, err
	}

	seasons, err := s.GetSeasonsByLeague(ctx, league)
	if err != nil {
		return nil, 0, nil, err
	}

	return matches, totalCount, seasons, nil
}

func (s *service) GetTeamWins(ctx context.Context, league string) ([]models.TeamWinsItem, error) {
	query := `
		WITH delivery_std AS (
			SELECT
				d.*,
				CASE
					WHEN d.batting_team = 'Royal Challengers Bengaluru' THEN 'Royal Challengers Bangalore'
					WHEN d.batting_team = 'Delhi Daredevils' THEN 'Delhi Capitals'
					WHEN d.batting_team = 'Kings XI Punjab' THEN 'Punjab Kings'
					WHEN d.batting_team = 'Rising Pune Supergiants' THEN 'Rising Pune Supergiant'
					ELSE d.batting_team
				END AS std_batting_team
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE m.league = $1 AND d.innings <= 2
		),
		runs_per_innings AS (
			SELECT
				match_id,
				innings,
				std_batting_team AS team,
				SUM(runs_off_bat + extras) AS runs
			FROM delivery_std
			GROUP BY match_id, innings, std_batting_team
		),
		match_totals AS (
			SELECT
				r1.match_id,
				r1.team AS team1,
				r1.runs AS runs1,
				r2.team AS team2,
				r2.runs AS runs2
			FROM runs_per_innings r1
			JOIN runs_per_innings r2
				ON r1.match_id = r2.match_id
				AND r1.innings = 1
				AND r2.innings = 2
		),
		winners AS (
			SELECT
				match_id,
				CASE WHEN runs1 > runs2 THEN team1 ELSE team2 END AS winner,
				CASE WHEN runs1 > runs2 THEN team2 ELSE team1 END AS loser,
				CASE WHEN runs1 > runs2 THEN 'batting_first' ELSE 'batting_second' END AS win_type
			FROM match_totals
		),
		teams AS (
			SELECT match_id, team1 AS team FROM match_totals
			UNION ALL
			SELECT match_id, team2 AS team FROM match_totals
		)
		SELECT
			t.team,
			COUNT(*) AS matches_played,
			COUNT(*) FILTER (WHERE t.team = w.winner) AS wins,
			COUNT(*) FILTER (WHERE t.team <> w.winner) AS losses,
			COUNT(*) FILTER (WHERE t.team = w.winner AND w.win_type = 'batting_first') AS wins_batting_first,
			COUNT(*) FILTER (WHERE t.team = w.winner AND w.win_type = 'batting_second') AS wins_batting_second
		FROM teams t
		LEFT JOIN winners w USING (match_id)
		GROUP BY t.team
		ORDER BY wins DESC, matches_played DESC;
	`

	rows, err := s.db.QueryContext(ctx, query, league)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TeamWinsItem
	for rows.Next() {
		var item models.TeamWinsItem
		if err := rows.Scan(
			&item.Team,
			&item.MatchesPlayed,
			&item.Wins,
			&item.Losses,
			&item.WinsBattingFirst,
			&item.WinsBattingSecond,
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

func (s *service) GetTeamAverages(ctx context.Context, league string) ([]models.TeamAverageItem, error) {
	query := `
		WITH standardized_deliveries AS (
			SELECT
				CASE
					WHEN d.batting_team = 'Royal Challengers Bengaluru' THEN 'Royal Challengers Bangalore'
					WHEN d.batting_team = 'Delhi Daredevils' THEN 'Delhi Capitals'
					WHEN d.batting_team = 'Kings XI Punjab' THEN 'Punjab Kings'
					WHEN d.batting_team = 'Rising Pune Supergiants' THEN 'Rising Pune Supergiant'
					ELSE d.batting_team
				END as team,
				d.match_id,
				d.innings,
				d.runs_off_bat,
				d.extras,
				d.wides,
				d.player_dismissed,
				d.wicket_type
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE d.innings <= 2
				AND m.league = $1
		),
		team_innings AS (
			SELECT
				team,
				match_id,
				innings,
				SUM(runs_off_bat + extras) as innings_total_runs
			FROM standardized_deliveries
			GROUP BY team, match_id, innings
		),
		team_stats AS (
			SELECT
				team,
				SUM(runs_off_bat) as total_runs,
				COUNT(*) FILTER (WHERE wides = 0) as total_balls,
				COUNT(*) FILTER (
					WHERE player_dismissed IS NOT NULL
					AND wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket', 'run out')
				) as total_dismissals
			FROM standardized_deliveries
			GROUP BY team
		)
		SELECT
			ti.team,
			COUNT(*) as total_innings,
			ts.total_runs,
			ts.total_balls,
			ts.total_dismissals,
			CASE
				WHEN ts.total_dismissals > 0
				THEN ts.total_runs::decimal / ts.total_dismissals
				ELSE ts.total_runs::decimal / NULLIF(COUNT(*), 0)
			END as batting_average,
			CASE
				WHEN ts.total_balls > 0
				THEN (ts.total_runs::decimal / ts.total_balls) * 100
				ELSE 0
			END as strike_rate,
			MAX(ti.innings_total_runs) as highest_score,
			MIN(ti.innings_total_runs) as lowest_score
		FROM team_innings ti
		JOIN team_stats ts ON ti.team = ts.team
		GROUP BY ti.team, ts.total_runs, ts.total_balls, ts.total_dismissals
		ORDER BY batting_average DESC;
	`

	rows, err := s.db.QueryContext(ctx, query, league)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.TeamAverageItem
	for rows.Next() {
		var item models.TeamAverageItem
		if err := rows.Scan(
			&item.Team,
			&item.TotalInnings,
			&item.TotalRuns,
			&item.TotalBalls,
			&item.TotalDismissals,
			&item.BattingAverage,
			&item.StrikeRate,
			&item.HighestScore,
			&item.LowestScore,
		); err != nil {
			return nil, err
		}

		item.BattingAverage = float64(int(item.BattingAverage*100)) / 100
		item.StrikeRate = float64(int(item.StrikeRate*100)) / 100
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func formatMatchResult(
	winner sql.NullString,
	winnerRuns sql.NullInt64,
	winnerWickets sql.NullInt64,
	team1 sql.NullString,
	team2 sql.NullString,
	innings1Score int,
	innings2Score int,
	innings2Wickets int,
) string {
	if winner.Valid && winner.String != "" {
		if winnerRuns.Valid && winnerRuns.Int64 > 0 {
			return fmt.Sprintf("%s won by %d runs", winner.String, winnerRuns.Int64)
		}
		if winnerWickets.Valid && winnerWickets.Int64 > 0 {
			return fmt.Sprintf("%s won by %d wickets", winner.String, winnerWickets.Int64)
		}
		return fmt.Sprintf("%s won", winner.String)
	}

	if innings1Score == innings2Score {
		return "Match Tied"
	}

	if innings1Score > innings2Score {
		return fmt.Sprintf("%s won by %d runs", team1.String, innings1Score-innings2Score)
	}

	return fmt.Sprintf("%s won by %d wickets", team2.String, 10-innings2Wickets)
}
