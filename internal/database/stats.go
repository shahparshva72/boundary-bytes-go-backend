package database

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

const standardizedBattingTeamSQL = `
	CASE
		WHEN d.batting_team = 'Royal Challengers Bengaluru' THEN 'Royal Challengers Bangalore'
		WHEN d.batting_team = 'Delhi Daredevils' THEN 'Delhi Capitals'
		WHEN d.batting_team = 'Kings XI Punjab' THEN 'Punjab Kings'
		WHEN d.batting_team = 'Rising Pune Supergiants' THEN 'Rising Pune Supergiant'
		ELSE d.batting_team
	END`

func (s *service) GetBowlingWicketTypes(
	ctx context.Context,
	league string,
	page int,
	limit int,
) ([]models.BowlingWicketTypesItem, int, error) {
	if page < 1 {
		page = 1
	}
	if limit < 1 {
		limit = 1
	}

	offset := (page - 1) * limit

	query := `
		SELECT
			d.bowler,
			COUNT(*) FILTER (WHERE d.wicket_type = 'caught') as caught,
			COUNT(*) FILTER (WHERE d.wicket_type = 'bowled') as bowled,
			COUNT(*) FILTER (WHERE d.wicket_type = 'lbw') as lbw,
			COUNT(*) FILTER (WHERE d.wicket_type = 'stumped') as stumped,
			COUNT(*) FILTER (WHERE d.wicket_type = 'caught and bowled') as caught_and_bowled,
			COUNT(*) FILTER (WHERE d.wicket_type = 'hit wicket') as hit_wicket,
			COUNT(*) FILTER (
				WHERE d.player_dismissed IS NOT NULL
				AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')
			) as total_wickets,
			COUNT(DISTINCT d.match_id) as matches
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		WHERE m.league = $1 AND d.innings <= 2
		GROUP BY d.bowler
		HAVING COUNT(*) FILTER (
			WHERE d.player_dismissed IS NOT NULL
			AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')
		) > 0
		ORDER BY total_wickets DESC
		LIMIT $2
		OFFSET $3;
	`

	rows, err := s.db.QueryContext(ctx, query, league, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var items []models.BowlingWicketTypesItem
	for rows.Next() {
		var (
			bowler       string
			caught       int
			bowled       int
			lbw          int
			stumped      int
			caughtBowled int
			hitWicket    int
			totalWickets int
			matches      int
		)
		if err := rows.Scan(
			&bowler,
			&caught,
			&bowled,
			&lbw,
			&stumped,
			&caughtBowled,
			&hitWicket,
			&totalWickets,
			&matches,
		); err != nil {
			return nil, 0, err
		}

		items = append(items, models.BowlingWicketTypesItem{
			Player:       bowler,
			TotalWickets: totalWickets,
			WicketTypes: models.BowlingWicketTypeBreakdown{
				Caught:          caught,
				Bowled:          bowled,
				Lbw:             lbw,
				Stumped:         stumped,
				CaughtAndBowled: caughtBowled,
				HitWicket:       hitWicket,
			},
			Matches: matches,
		})
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	countQuery := `
		SELECT COUNT(T.bowler)
		FROM (
			SELECT d.bowler
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE m.league = $1
			GROUP BY d.bowler
			HAVING COUNT(*) FILTER (
				WHERE d.player_dismissed IS NOT NULL
				AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')
			) > 0
		) AS T;
	`

	var totalCount int
	if err := s.db.QueryRowContext(ctx, countQuery, league).Scan(&totalCount); err != nil {
		return nil, 0, err
	}

	return items, totalCount, nil
}

func (s *service) GetMultiMatchup(
	ctx context.Context,
	league string,
	player string,
	opponents []string,
	mode string,
) ([]models.MultiMatchupItem, error) {
	if len(opponents) == 0 {
		return nil, errors.New("at least one opponent is required")
	}

	var query string
	var opponentClause string
	args := []interface{}{league, player}

	if mode == "batterVsBowlers" {
		opponentClause = buildInClause(len(opponents), 3)
		for _, opponent := range opponents {
			args = append(args, opponent)
		}
		query = fmt.Sprintf(`
			SELECT
				d.bowler as opponent,
				COALESCE(SUM(d.runs_off_bat), 0)::int as runs_scored,
				COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)::int as balls_faced,
				COUNT(CASE WHEN d.player_dismissed = $2 THEN 1 END)::int as dismissals,
				CASE
					WHEN COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0) > 0
					THEN ROUND((COALESCE(SUM(d.runs_off_bat), 0)::numeric / COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)) * 100, 2)
					ELSE 0
				END as strike_rate,
				0 as economy_rate,
				CASE
					WHEN COUNT(CASE WHEN d.player_dismissed = $2 THEN 1 END) > 0
					THEN ROUND(COALESCE(SUM(d.runs_off_bat), 0)::numeric / COUNT(CASE WHEN d.player_dismissed = $2 THEN 1 END), 2)
					ELSE COALESCE(SUM(d.runs_off_bat), 0)::numeric
				END as average,
				COUNT(CASE WHEN d.runs_off_bat = 4 THEN 1 END)::int as fours,
				COUNT(CASE WHEN d.runs_off_bat = 6 THEN 1 END)::int as sixes,
				0 as dot_balls
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE d.striker = $2
				AND d.bowler IN %s
				AND m.league = $1
				AND d.innings <= 2
			GROUP BY d.bowler
			ORDER BY runs_scored DESC;
		`, opponentClause)
	} else {
		opponentClause = buildInClause(len(opponents), 3)
		for _, opponent := range opponents {
			args = append(args, opponent)
		}
		query = fmt.Sprintf(`
			SELECT
				d.striker as opponent,
				COALESCE(SUM(d.runs_off_bat), 0)::int as runs_scored,
				COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)::int as balls_faced,
				COUNT(CASE WHEN d.player_dismissed IS NOT NULL THEN 1 END)::int as dismissals,
				0 as strike_rate,
				CASE
					WHEN COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0) > 0
					THEN ROUND((COALESCE(SUM(d.runs_off_bat), 0)::numeric / (COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)::numeric / 6)), 2)
					ELSE 0
				END as economy_rate,
				CASE
					WHEN COUNT(CASE WHEN d.player_dismissed IS NOT NULL THEN 1 END) > 0
					THEN ROUND(COALESCE(SUM(d.runs_off_bat), 0)::numeric / COUNT(CASE WHEN d.player_dismissed IS NOT NULL THEN 1 END), 2)
					ELSE COALESCE(SUM(d.runs_off_bat), 0)::numeric
				END as average,
				COUNT(CASE WHEN d.runs_off_bat = 4 THEN 1 END)::int as fours,
				COUNT(CASE WHEN d.runs_off_bat = 6 THEN 1 END)::int as sixes,
				COUNT(CASE WHEN d.runs_off_bat = 0 AND d.extras = 0 THEN 1 END)::int as dot_balls
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE d.bowler = $2
				AND d.striker IN %s
				AND m.league = $1
				AND d.innings <= 2
			GROUP BY d.striker
			ORDER BY dismissals DESC, economy_rate ASC;
		`, opponentClause)
	}

	rows, err := s.db.QueryContext(ctx, query, args...)
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
			&item.EconomyRate,
			&item.Average,
			&item.Fours,
			&item.Sixes,
			&item.DotBalls,
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

func (s *service) GetRunRateTrend(ctx context.Context, league string, team *string) ([]models.RunRateTrendItem, error) {
	seasonFloorClause := ""
	if league == "IPL" {
		seasonFloorClause = "AND m.season >= '2008'"
	}

	args := []interface{}{league}
	teamClause := ""
	if team != nil && *team != "" {
		args = append(args, *team)
		teamClause = fmt.Sprintf("AND %s = $%d", standardizedBattingTeamSQL, len(args))
	}

	query := fmt.Sprintf(`
		SELECT
			m.season,
			SUM(d.runs_off_bat + d.extras)::int AS total_runs,
			SUM(CASE WHEN d.wides = 0 OR d.wides IS NULL THEN 1 ELSE 0 END)::int AS total_balls
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		WHERE d.innings <= 2
			AND m.league = $1
			%s
			%s
		GROUP BY m.season
		ORDER BY m.season ASC;
	`, seasonFloorClause, teamClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := []models.RunRateTrendItem{}
	for rows.Next() {
		var item models.RunRateTrendItem
		if err := rows.Scan(&item.Season, &item.TotalRuns, &item.TotalBalls); err != nil {
			return nil, err
		}
		if item.TotalBalls > 0 {
			item.AvgRunRate = math.Round((float64(item.TotalRuns)/(float64(item.TotalBalls)/6))*100) / 100
		}
		items = append(items, item)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return items, nil
}

func (s *service) GetTeamRunRateProgression(
	ctx context.Context,
	league string,
	team string,
	season string,
) ([]models.TeamRunRateProgressionPoint, models.TeamRunRateProgressionMetadata, error) {
	aggregatedQuery := fmt.Sprintf(`
		WITH standardized_deliveries AS (
			SELECT
				%s as team,
				d.match_id,
				d.innings,
				d.ball,
				d.runs_off_bat,
				d.extras,
				d.wides
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE d.innings <= 2
				AND m.league = $1
				AND m.season = $2
		)
		SELECT
			FLOOR(CAST(ball AS DECIMAL(10,2)))::int + 1 AS over_number,
			SUM(runs_off_bat + extras)::int AS runs,
			SUM(CASE WHEN wides = 0 OR wides IS NULL THEN 1 ELSE 0 END)::int AS balls
		FROM standardized_deliveries
		WHERE team = $3
			AND FLOOR(CAST(ball AS DECIMAL(10,2)))::int + 1 BETWEEN 1 AND 20
		GROUP BY over_number
		ORDER BY over_number;
	`, standardizedBattingTeamSQL)

	rows, err := s.db.QueryContext(ctx, aggregatedQuery, league, season, team)
	if err != nil {
		return nil, models.TeamRunRateProgressionMetadata{}, err
	}
	defer rows.Close()

	overData := map[int]struct {
		runs  int
		balls int
	}{}
	for rows.Next() {
		var over int
		var runs int
		var balls int
		if err := rows.Scan(&over, &runs, &balls); err != nil {
			return nil, models.TeamRunRateProgressionMetadata{}, err
		}
		overData[over] = struct {
			runs  int
			balls int
		}{runs: runs, balls: balls}
	}

	if err := rows.Err(); err != nil {
		return nil, models.TeamRunRateProgressionMetadata{}, err
	}

	metadataQuery := fmt.Sprintf(`
		WITH standardized_deliveries AS (
			SELECT
				%s as team,
				d.match_id,
				d.innings,
				d.ball
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE d.innings <= 2
				AND m.league = $1
				AND m.season = $2
		)
		SELECT
			COUNT(DISTINCT CONCAT(match_id, '-', innings))::int AS total_innings,
			COUNT(DISTINCT match_id)::int AS total_matches,
			COUNT(*)::int AS total_deliveries
		FROM standardized_deliveries
		WHERE team = $3;
	`, standardizedBattingTeamSQL)

	metadata := models.TeamRunRateProgressionMetadata{}
	if err := s.db.QueryRowContext(ctx, metadataQuery, league, season, team).Scan(
		&metadata.TotalInnings,
		&metadata.TotalMatches,
		&metadata.TotalDeliveries,
	); err != nil {
		return nil, models.TeamRunRateProgressionMetadata{}, err
	}

	progression := make([]models.TeamRunRateProgressionPoint, 0, 20)
	for over := 1; over <= 20; over++ {
		data := overData[over]
		runRate := 0.0
		if data.balls > 0 {
			runRate = float64(data.runs) / (float64(data.balls) / 6)
		}

		progression = append(progression, models.TeamRunRateProgressionPoint{
			Over:    over,
			Phase:   phaseForOver(over),
			Runs:    data.runs,
			Balls:   data.balls,
			RunRate: math.Round(runRate*100) / 100,
		})
	}

	return progression, metadata, nil
}

func (s *service) GetPlayerCompare(
	ctx context.Context,
	league string,
	players []string,
	seasons []string,
	team *string,
	statType string,
) ([]models.PlayerComparePlayer, error) {
	comparedPlayers := make([]models.PlayerComparePlayer, len(players))
	playerIndex := make(map[string]int, len(players))
	for index, player := range players {
		comparedPlayers[index] = models.PlayerComparePlayer{Name: player}
		playerIndex[player] = index
	}

	if statType == "batting" || statType == "both" {
		battingStats, err := s.getPlayerCompareBatting(ctx, league, players, seasons, team)
		if err != nil {
			return nil, err
		}
		for _, stat := range battingStats {
			index, ok := playerIndex[stat.Player]
			if !ok {
				continue
			}

			runs := stat.Runs
			ballsFaced := stat.BallsFaced
			innings := stat.Innings
			notOuts := stat.NotOuts
			dismissals := innings - notOuts

			batting := &models.PlayerCompareBatting{
				Runs:         runs,
				BallsFaced:   ballsFaced,
				Innings:      innings,
				NotOuts:      notOuts,
				HighestScore: stat.HighestScore,
				Fours:        stat.Fours,
				Sixes:        stat.Sixes,
				Fifties:      stat.Fifties,
				Hundreds:     stat.Hundreds,
			}
			if ballsFaced > 0 {
				batting.StrikeRate = math.Round((float64(runs)/float64(ballsFaced))*10000) / 100
			}
			if dismissals > 0 {
				batting.Average = math.Round((float64(runs)/float64(dismissals))*100) / 100
			} else {
				batting.Average = float64(runs)
			}

			comparedPlayers[index].Batting = batting
		}
	}

	if statType == "bowling" || statType == "both" {
		bowlingStats, err := s.getPlayerCompareBowling(ctx, league, players, seasons, team)
		if err != nil {
			return nil, err
		}
		for _, stat := range bowlingStats {
			index, ok := playerIndex[stat.Player]
			if !ok {
				continue
			}

			bowling := &models.PlayerCompareBowling{
				Wickets:      stat.Wickets,
				BallsBowled:  stat.BallsBowled,
				RunsConceded: stat.RunsConceded,
				Innings:      stat.Innings,
				FourWickets:  stat.FourWickets,
				FiveWickets:  stat.FiveWickets,
			}
			if stat.BallsBowled > 0 {
				bowling.Economy = math.Round((float64(stat.RunsConceded)/(float64(stat.BallsBowled)/6))*100) / 100
			}
			if stat.Wickets > 0 {
				bowling.Average = math.Round((float64(stat.RunsConceded)/float64(stat.Wickets))*100) / 100
				bowling.StrikeRate = math.Round((float64(stat.BallsBowled)/float64(stat.Wickets))*100) / 100
			}

			comparedPlayers[index].Bowling = bowling
		}
	}

	return comparedPlayers, nil
}

func (s *service) GetStatExplorerOptions(
	ctx context.Context,
	league string,
	reportType string,
) (models.StatExplorerFilterOptions, error) {
	teams, err := s.queryStringList(ctx, fmt.Sprintf(`
		SELECT DISTINCT %s AS team
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		WHERE m.league = $1
		ORDER BY team;
	`, standardizedBattingTeamSQL), league)
	if err != nil {
		return models.StatExplorerFilterOptions{}, err
	}

	seasons, err := s.queryStringList(ctx, `
		SELECT DISTINCT season
		FROM wpl_match
		WHERE league = $1
		ORDER BY season DESC;
	`, league)
	if err != nil {
		return models.StatExplorerFilterOptions{}, err
	}

	venues, err := s.queryStringList(ctx, `
		SELECT DISTINCT venue
		FROM wpl_match
		WHERE league = $1
		ORDER BY venue;
	`, league)
	if err != nil {
		return models.StatExplorerFilterOptions{}, err
	}

	cities, err := s.queryStringList(ctx, `
		SELECT DISTINCT city
		FROM wpl_match_info
		WHERE league = $1
		ORDER BY city;
	`, league)
	if err != nil {
		return models.StatExplorerFilterOptions{}, err
	}

	tossWinners, err := s.queryStringList(ctx, `
		SELECT DISTINCT toss_winner
		FROM wpl_match_info
		WHERE league = $1
		ORDER BY toss_winner;
	`, league)
	if err != nil {
		return models.StatExplorerFilterOptions{}, err
	}

	return models.StatExplorerFilterOptions{
		Teams:               teams,
		Opposition:          teams,
		Seasons:             seasons,
		Venues:              venues,
		Cities:              cities,
		TossWinners:         tossWinners,
		TossDecisions:       []string{"bat", "field"},
		Innings:             []int{1, 2},
		AvailableMetrics:    statExplorerAllowedMetrics(reportType),
		AvailableDimensions: statExplorerAllowedDimensions(reportType),
		BattingHands:        []string{"right", "left"},
		BowlingTypes:        []string{"pace", "spin"},
		BowlingSubTypes: []string{
			"fast",
			"fast-medium",
			"medium-fast",
			"medium",
			"offbreak",
			"legbreak",
			"left-arm-orthodox",
			"left-arm-wrist-spin",
			"slow",
		},
		PlayingRoles: []string{"batter", "bowler", "allrounder", "wicketkeeper"},
		PlayingRoleDetails: []string{
			"opening_batter",
			"top_order_batter",
			"middle_order_batter",
			"batter",
			"batting_allrounder",
			"bowling_allrounder",
			"allrounder",
			"bowler",
			"wicketkeeper_batter",
			"wicketkeeper",
		},
	}, nil
}

func (s *service) getPlayerCompareBatting(
	ctx context.Context,
	league string,
	players []string,
	seasons []string,
	team *string,
) ([]playerCompareBattingRow, error) {
	args := []interface{}{league}
	argIndex := 2
	playerClause := buildInClause(len(players), argIndex)
	for _, player := range players {
		args = append(args, player)
	}
	argIndex += len(players)

	seasonClause := ""
	if len(seasons) > 0 {
		seasonClause = fmt.Sprintf("AND m.season IN %s", buildInClause(len(seasons), argIndex))
		for _, season := range seasons {
			args = append(args, season)
		}
		argIndex += len(seasons)
	}

	teamClause := ""
	if team != nil && *team != "" {
		teamClause = fmt.Sprintf("AND d.batting_team = $%d", argIndex)
		args = append(args, *team)
	}

	query := fmt.Sprintf(`
		WITH innings_data AS (
			SELECT
				d.striker,
				d.match_id,
				d.innings,
				SUM(d.runs_off_bat)::int as innings_runs,
				COUNT(*) FILTER (WHERE d.runs_off_bat = 4)::int as fours,
				COUNT(*) FILTER (WHERE d.runs_off_bat = 6)::int as sixes,
				COUNT(*) FILTER (WHERE d.wides = 0)::int as balls_faced
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE d.striker IN %s
				AND m.league = $1
				AND d.innings <= 2
				%s
				%s
			GROUP BY d.striker, d.match_id, d.innings
		),
		dismissals_data AS (
			SELECT
				d.player_dismissed as player,
				d.match_id,
				d.innings,
				1 as was_out
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE d.player_dismissed IN %s
				AND m.league = $1
				AND d.innings <= 2
				AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket', 'run out', 'retired out', 'obstructing the field', 'hit the ball twice', 'handled the ball', 'timed out')
				%s
				%s
			GROUP BY d.player_dismissed, d.match_id, d.innings
		),
		combined_innings AS (
			SELECT
				i.striker,
				i.match_id,
				i.innings,
				i.innings_runs,
				i.fours,
				i.sixes,
				i.balls_faced,
				COALESCE(dd.was_out, 0) as was_out
			FROM innings_data i
			LEFT JOIN dismissals_data dd
				ON i.striker = dd.player AND i.match_id = dd.match_id AND i.innings = dd.innings
		),
		innings_agg AS (
			SELECT
				striker,
				COUNT(*)::int as innings,
				COUNT(*) FILTER (WHERE was_out = 0)::int as not_outs,
				MAX(innings_runs)::int as highest_score,
				COUNT(*) FILTER (WHERE innings_runs >= 50 AND innings_runs < 100)::int as fifties,
				COUNT(*) FILTER (WHERE innings_runs >= 100)::int as hundreds
			FROM combined_innings
			GROUP BY striker
		)
		SELECT
			c.striker,
			COALESCE(SUM(c.innings_runs), 0)::int as runs,
			COALESCE(SUM(c.balls_faced), 0)::int as balls_faced,
			COALESCE(ia.innings, 0)::int as innings,
			COALESCE(ia.not_outs, 0)::int as not_outs,
			COALESCE(ia.highest_score, 0)::int as highest_score,
			COALESCE(SUM(c.fours), 0)::int as fours,
			COALESCE(SUM(c.sixes), 0)::int as sixes,
			COALESCE(ia.fifties, 0)::int as fifties,
			COALESCE(ia.hundreds, 0)::int as hundreds
		FROM combined_innings c
		LEFT JOIN innings_agg ia ON c.striker = ia.striker
		GROUP BY c.striker, ia.innings, ia.not_outs, ia.highest_score, ia.fifties, ia.hundreds;
	`, playerClause, seasonClause, teamClause, playerClause, seasonClause, teamClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := []playerCompareBattingRow{}
	for rows.Next() {
		var stat playerCompareBattingRow
		if err := rows.Scan(
			&stat.Player,
			&stat.Runs,
			&stat.BallsFaced,
			&stat.Innings,
			&stat.NotOuts,
			&stat.HighestScore,
			&stat.Fours,
			&stat.Sixes,
			&stat.Fifties,
			&stat.Hundreds,
		); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func (s *service) queryStringList(ctx context.Context, query string, args ...interface{}) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	values := []string{}
	for rows.Next() {
		var value sql.NullString
		if err := rows.Scan(&value); err != nil {
			return nil, err
		}
		if value.Valid {
			values = append(values, value.String)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return values, nil
}

func (s *service) getPlayerCompareBowling(
	ctx context.Context,
	league string,
	players []string,
	seasons []string,
	team *string,
) ([]playerCompareBowlingRow, error) {
	args := []interface{}{league}
	argIndex := 2
	playerClause := buildInClause(len(players), argIndex)
	for _, player := range players {
		args = append(args, player)
	}
	argIndex += len(players)

	seasonClause := ""
	if len(seasons) > 0 {
		seasonClause = fmt.Sprintf("AND m.season IN %s", buildInClause(len(seasons), argIndex))
		for _, season := range seasons {
			args = append(args, season)
		}
		argIndex += len(seasons)
	}

	teamClause := ""
	if team != nil && *team != "" {
		teamClause = fmt.Sprintf("AND d.bowling_team = $%d", argIndex)
		args = append(args, *team)
	}

	query := fmt.Sprintf(`
		WITH bowling_innings AS (
			SELECT
				d.bowler,
				d.match_id,
				d.innings,
				COUNT(*) FILTER (
					WHERE d.player_dismissed IS NOT NULL
					AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')
				)::int as wickets_in_innings
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE d.bowler IN %s
				AND m.league = $1
				AND d.innings <= 2
				%s
				%s
			GROUP BY d.bowler, d.match_id, d.innings
		),
		innings_agg AS (
			SELECT
				bowler,
				COUNT(*)::int as innings,
				COUNT(*) FILTER (WHERE wickets_in_innings >= 4 AND wickets_in_innings < 5)::int as four_wickets,
				COUNT(*) FILTER (WHERE wickets_in_innings >= 5)::int as five_wickets
			FROM bowling_innings
			GROUP BY bowler
		)
		SELECT
			d.bowler,
			COUNT(*) FILTER (
				WHERE d.player_dismissed IS NOT NULL
				AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')
			)::int as wickets,
			COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)::int as balls_bowled,
			COALESCE(SUM(d.runs_off_bat + d.wides + d.noballs), 0)::int as runs_conceded,
			COALESCE(ia.innings, 0)::int as innings,
			COALESCE(ia.four_wickets, 0)::int as four_wickets,
			COALESCE(ia.five_wickets, 0)::int as five_wickets
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		LEFT JOIN innings_agg ia ON d.bowler = ia.bowler
		WHERE d.bowler IN %s
			AND m.league = $1
			AND d.innings <= 2
			%s
			%s
		GROUP BY d.bowler, ia.innings, ia.four_wickets, ia.five_wickets;
	`, playerClause, seasonClause, teamClause, playerClause, seasonClause, teamClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := []playerCompareBowlingRow{}
	for rows.Next() {
		var stat playerCompareBowlingRow
		if err := rows.Scan(
			&stat.Player,
			&stat.Wickets,
			&stat.BallsBowled,
			&stat.RunsConceded,
			&stat.Innings,
			&stat.FourWickets,
			&stat.FiveWickets,
		); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return stats, nil
}

func (s *service) GetPlayerProgression(
	ctx context.Context,
	league string,
	player string,
	innings *int,
) ([]models.PlayerProgressionPoint, models.PlayerProgressionMetadata, error) {
	inningsClause := "AND d.innings <= 2"
	args := []interface{}{player, league}
	if innings != nil && (*innings == 1 || *innings == 2) {
		inningsClause = "AND d.innings = $3"
		args = append(args, *innings)
	}

	query := fmt.Sprintf(`
		SELECT
			FLOOR(CAST(d.ball AS DECIMAL(10,2)))::int + 1 AS over_number,
			SUM(d.runs_off_bat) AS runs,
			SUM(CASE WHEN d.wides = 0 OR d.wides IS NULL THEN 1 ELSE 0 END) AS balls,
			SUM(CASE WHEN d.player_dismissed = $1 THEN 1 ELSE 0 END) AS dismissals
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		WHERE d.striker = $1
			AND m.league = $2
			%s
			AND FLOOR(CAST(d.ball AS DECIMAL(10,2)))::int + 1 BETWEEN 1 AND 20
		GROUP BY FLOOR(CAST(d.ball AS DECIMAL(10,2)))::int + 1
		ORDER BY over_number;
	`, inningsClause)

	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, models.PlayerProgressionMetadata{}, err
	}
	defer rows.Close()

	dataMap := map[int]struct {
		runs       int
		balls      int
		dismissals int
	}{}
	for rows.Next() {
		var over int
		var runs int
		var balls int
		var dismissals int
		if err := rows.Scan(&over, &runs, &balls, &dismissals); err != nil {
			return nil, models.PlayerProgressionMetadata{}, err
		}
		dataMap[over] = struct {
			runs       int
			balls      int
			dismissals int
		}{runs: runs, balls: balls, dismissals: dismissals}
	}

	if err := rows.Err(); err != nil {
		return nil, models.PlayerProgressionMetadata{}, err
	}

	progression := make([]models.PlayerProgressionPoint, 0, 20)
	for over := 1; over <= 20; over++ {
		d := dataMap[over]
		strikeRate := 0.0
		if d.balls > 0 {
			strikeRate = (float64(d.runs) / float64(d.balls)) * 100
		}
		var avgPtr *float64
		if d.dismissals > 0 {
			avg := float64(d.runs) / float64(d.dismissals)
			avgRounded := math.Round(avg*100) / 100
			avgPtr = &avgRounded
		}

		progression = append(progression, models.PlayerProgressionPoint{
			Over:       over,
			Phase:      phaseForOver(over),
			Runs:       d.runs,
			Balls:      d.balls,
			Dismissals: d.dismissals,
			StrikeRate: math.Round(strikeRate*100) / 100,
			Average:    avgPtr,
		})
	}

	metaQuery := fmt.Sprintf(`
		SELECT
			COUNT(DISTINCT CONCAT(d.match_id, '-', d.innings)) AS total_innings,
			COUNT(DISTINCT d.match_id) AS total_matches,
			COUNT(*) AS total_deliveries
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		WHERE d.striker = $1
			AND m.league = $2
			%s;
	`, inningsClause)

	var (
		totalInnings    int
		totalMatches    int
		totalDeliveries int
	)

	if err := s.db.QueryRowContext(ctx, metaQuery, args...).Scan(&totalInnings, &totalMatches, &totalDeliveries); err != nil {
		return nil, models.PlayerProgressionMetadata{}, err
	}

	metadata := models.PlayerProgressionMetadata{
		TotalInnings:    totalInnings,
		TotalMatches:    totalMatches,
		TotalDeliveries: totalDeliveries,
	}

	return progression, metadata, nil
}

func (s *service) GetAdvancedStats(
	ctx context.Context,
	league string,
	playerType string,
	player string,
	overs []int,
) (interface{}, int, error) {
	if len(overs) == 0 {
		return nil, 0, errors.New("overs are required")
	}

	query := `
		SELECT
			d.runs_off_bat,
			d.extras,
			d.wides,
			d.noballs,
			d.player_dismissed,
			d.wicket_type,
			d.ball
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		WHERE m.league = $1
			AND d.innings <= 2
			AND %s = $2;
	`

	field := "d.striker"
	if playerType == "bowler" {
		field = "d.bowler"
	}

	query = fmt.Sprintf(query, field)

	rows, err := s.db.QueryContext(ctx, query, league, player)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	oversSet := map[int]bool{}
	for _, over := range overs {
		oversSet[over] = true
	}

	var deliveries []deliveryRow
	for rows.Next() {
		var d deliveryRow
		if err := rows.Scan(
			&d.runsOffBat,
			&d.extras,
			&d.wides,
			&d.noballs,
			&d.playerDismissed,
			&d.wicketType,
			&d.ball,
		); err != nil {
			return nil, 0, err
		}
		if overFromBall(d.ball, oversSet) {
			deliveries = append(deliveries, d)
		}
	}

	if err := rows.Err(); err != nil {
		return nil, 0, err
	}

	if playerType == "bowler" {
		return calculateAdvancedBowlerStats(deliveries), len(deliveries), nil
	}

	return calculateAdvancedBatterStats(deliveries, player), len(deliveries), nil
}

func (s *service) GetFallOfWickets(
	ctx context.Context,
	league string,
	matchID int,
) (*models.FallOfWicketsResponse, error) {
	matchQuery := `
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
				) as teams
			FROM wpl_match m
			JOIN wpl_delivery d ON d.match_id = m.match_id
			WHERE m.match_id = $1 AND m.league = $2
			GROUP BY m.match_id, m.league, m.season, m.start_date, m.venue
		)
		SELECT match_id, league, season, start_date, venue, teams FROM match_teams;
	`

	var (
		matchIDValue int
		matchLeague  string
		matchSeason  string
		matchDate    sql.NullTime
		venue        string
		teams        string
	)

	if err := s.db.QueryRowContext(ctx, matchQuery, matchID, league).Scan(
		&matchIDValue,
		&matchLeague,
		&matchSeason,
		&matchDate,
		&venue,
		&teams,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	fallQuery := `
		WITH wicket_details AS (
			SELECT
				d.match_id,
				d.innings,
				d.ball,
				d.player_dismissed,
				d.wicket_type,
				d.bowler,
				CASE
					WHEN d.batting_team = 'Royal Challengers Bengaluru' THEN 'Royal Challengers Bangalore'
					WHEN d.batting_team = 'Delhi Daredevils' THEN 'Delhi Capitals'
					WHEN d.batting_team = 'Kings XI Punjab' THEN 'Punjab Kings'
					WHEN d.batting_team = 'Rising Pune Supergiants' THEN 'Rising Pune Supergiant'
					ELSE d.batting_team
				END as batting_team,
				ROW_NUMBER() OVER (PARTITION BY d.match_id, d.innings ORDER BY d.ball) as wicket_number
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			WHERE d.player_dismissed IS NOT NULL
				AND d.match_id = $1
				AND m.league = $2
				AND d.innings <= 2
				AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket', 'run out', 'retired hurt', 'obstructing the field', 'hit the ball twice', 'handled the ball', 'timed out')
		),
		runs_at_wicket AS (
			SELECT
				wd.*,
				SUM(d.runs_off_bat + d.extras) as runs_at_fall
			FROM wicket_details wd
			JOIN wpl_delivery d ON d.match_id = wd.match_id
				AND d.innings = wd.innings
				AND d.ball <= wd.ball
			GROUP BY wd.match_id, wd.innings, wd.ball, wd.player_dismissed, wd.wicket_type, wd.bowler, wd.batting_team, wd.wicket_number
		)
		SELECT
			innings,
			batting_team,
			ball,
			player_dismissed,
			wicket_type,
			bowler,
			wicket_number,
			runs_at_fall
		FROM runs_at_wicket
		ORDER BY innings, wicket_number;
	`

	rows, err := s.db.QueryContext(ctx, fallQuery, matchID, league)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	inningsMap := map[int]*models.FallOfWicketsInnings{}
	count := 0
	for rows.Next() {
		var (
			innings         int
			battingTeam     string
			ball            string
			playerDismissed string
			wicketType      string
			bowler          string
			wicketNumber    int
			runsAtFall      int
		)
		if err := rows.Scan(
			&innings,
			&battingTeam,
			&ball,
			&playerDismissed,
			&wicketType,
			&bowler,
			&wicketNumber,
			&runsAtFall,
		); err != nil {
			return nil, err
		}

		entry := models.FallOfWicketsEntry{
			WicketNumber:  wicketNumber,
			Over:          ball,
			RunsAtFall:    runsAtFall,
			BatsmanOut:    playerDismissed,
			DismissalType: wicketType,
			Bowler:        bowler,
		}

		if inningsMap[innings] == nil {
			inningsMap[innings] = &models.FallOfWicketsInnings{
				InningsNumber: innings,
				BattingTeam:   battingTeam,
				Wickets:       []models.FallOfWicketsEntry{},
			}
		}

		inningsMap[innings].Wickets = append(inningsMap[innings].Wickets, entry)
		count++
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	innings := make([]models.FallOfWicketsInnings, 0, len(inningsMap))
	for index := 1; index <= 2; index++ {
		if inning, ok := inningsMap[index]; ok {
			innings = append(innings, *inning)
		}
	}

	date := ""
	if matchDate.Valid {
		date = matchDate.Time.Format("2006-01-02")
	}

	response := &models.FallOfWicketsResponse{
		MatchInfo: models.FallOfWicketsMatchInfo{
			ID:     matchIDValue,
			League: matchLeague,
			Teams:  strings.Split(teams, " vs "),
			Venue:  venue,
			Date:   date,
			Season: matchSeason,
		},
		Innings: innings,
		Metadata: models.FallOfWicketsMetadata{
			TotalWickets: count,
		},
	}

	return response, nil
}

func phaseForOver(over int) string {
	if over <= 6 {
		return "powerplay"
	}
	if over <= 15 {
		return "middle"
	}
	return "death"
}

func overFromBall(ball string, allowed map[int]bool) bool {
	if ball == "" {
		return false
	}
	parts := strings.Split(ball, ".")
	if len(parts) == 0 {
		return false
	}
	over, err := strconv.Atoi(parts[0])
	if err != nil {
		return false
	}
	return allowed[over+1]
}

func statExplorerAllowedDimensions(reportType string) []string {
	switch reportType {
	case "batting", "bowling":
		return []string{
			"season",
			"player",
			"team",
			"opposition",
			"venue",
			"city",
			"tossWinner",
			"tossDecision",
			"result",
			"date",
			"innings",
			"battingHand",
			"bowlingType",
			"bowlingSubType",
			"opponentBattingHand",
			"opponentBowlingType",
			"opponentBowlingSubType",
			"playingRole",
		}
	case "team":
		return []string{"season", "team", "venue", "city", "tossWinner", "tossDecision", "result", "date"}
	case "match":
		return []string{"season", "team", "opposition", "venue", "city", "tossWinner", "tossDecision", "result", "date", "innings"}
	default:
		return []string{}
	}
}

func statExplorerAllowedMetrics(reportType string) []string {
	switch reportType {
	case "batting":
		return []string{"runs", "ballsFaced", "innings", "notOuts", "highestScore", "fours", "sixes", "fifties", "hundreds", "strikeRate", "average", "dismissals", "dotBalls"}
	case "bowling":
		return []string{"wickets", "ballsBowled", "runsConceded", "innings", "economyRate", "bowlingAverage", "bowlingStrikeRate", "fourWickets", "fiveWickets", "dotBalls"}
	case "team":
		return []string{"matchesPlayed", "wins", "losses", "winPct", "winsBattingFirst", "winsBattingSecond"}
	case "match":
		return []string{"matches", "runs", "wickets", "ballsFaced", "ballsBowled", "economyRate", "strikeRate"}
	default:
		return []string{}
	}
}

func calculateAdvancedBatterStats(deliveries []deliveryRow, player string) models.AdvancedStatsBatterData {
	runsScored := 0
	ballsFaced := 0
	fours := 0
	sixes := 0
	dismissals := 0

	for _, d := range deliveries {
		runsScored += d.runsOffBat
		if d.wides == 0 {
			ballsFaced++
		}
		if d.runsOffBat == 4 {
			fours++
		}
		if d.runsOffBat == 6 {
			sixes++
		}
		if d.playerDismissed.Valid && d.playerDismissed.String == player {
			dismissals++
		}
	}

	strikeRate := 0.0
	if ballsFaced > 0 {
		strikeRate = (float64(runsScored) / float64(ballsFaced)) * 100
	}
	average := float64(runsScored)
	if dismissals > 0 {
		average = float64(runsScored) / float64(dismissals)
	}

	return models.AdvancedStatsBatterData{
		RunsScored: runsScored,
		BallsFaced: ballsFaced,
		StrikeRate: math.Round(strikeRate*100) / 100,
		Average:    math.Round(average*100) / 100,
		Fours:      fours,
		Sixes:      sixes,
		Dismissals: dismissals,
	}
}

func calculateAdvancedBowlerStats(deliveries []deliveryRow) models.AdvancedStatsBowlerData {
	runsConceded := 0
	ballsBowled := 0
	wickets := 0
	dots := 0
	wides := 0
	noballs := 0

	bowlerWicketTypes := map[string]bool{
		"bowled":            true,
		"caught":            true,
		"lbw":               true,
		"stumped":           true,
		"caught and bowled": true,
		"hit wicket":        true,
	}

	for _, d := range deliveries {
		runsConceded += d.runsOffBat
		if d.wides > 0 {
			runsConceded += d.wides
			wides++
		}
		if d.noballs > 0 {
			runsConceded += d.noballs
			noballs++
		}
		if d.wides == 0 {
			ballsBowled++
			if d.runsOffBat == 0 && d.extras == 0 {
				dots++
			}
		}
		if d.playerDismissed.Valid && d.wicketType.Valid {
			if bowlerWicketTypes[d.wicketType.String] {
				wickets++
			}
		}
	}

	overs := float64(ballsBowled/6) + float64(ballsBowled%6)/10
	economyRate := 0.0
	if overs > 0 {
		economyRate = float64(runsConceded) / overs
	}
	average := 0.0
	strikeRate := 0.0
	if wickets > 0 {
		average = float64(runsConceded) / float64(wickets)
		strikeRate = float64(ballsBowled) / float64(wickets)
	}

	return models.AdvancedStatsBowlerData{
		RunsConceded: runsConceded,
		BallsBowled:  ballsBowled,
		Overs:        math.Round(overs*10) / 10,
		Wickets:      wickets,
		EconomyRate:  math.Round(economyRate*100) / 100,
		Average:      math.Round(average*100) / 100,
		StrikeRate:   math.Round(strikeRate*100) / 100,
		Dots:         dots,
		Wides:        wides,
		Noballs:      noballs,
	}
}

func pqStringArray(values []string) interface{} {
	return fmt.Sprintf("{%s}", strings.Join(values, ","))
}

func buildInClause(count int, startIndex int) string {
	placeholders := make([]string, 0, count)
	for i := 0; i < count; i++ {
		placeholders = append(placeholders, fmt.Sprintf("$%d", startIndex+i))
	}
	return fmt.Sprintf("(%s)", strings.Join(placeholders, ","))
}

type deliveryRow struct {
	runsOffBat      int
	extras          int
	wides           int
	noballs         int
	playerDismissed sql.NullString
	wicketType      sql.NullString
	ball            string
}

type playerCompareBattingRow struct {
	Player       string
	Runs         int
	BallsFaced   int
	Innings      int
	NotOuts      int
	HighestScore int
	Fours        int
	Sixes        int
	Fifties      int
	Hundreds     int
}

type playerCompareBowlingRow struct {
	Player       string
	Wickets      int
	BallsBowled  int
	RunsConceded int
	Innings      int
	FourWickets  int
	FiveWickets  int
}
