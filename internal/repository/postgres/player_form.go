package postgres

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

const (
	bowlerWicketTypesSQL     = "('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')"
	battingDismissalTypesSQL = "('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket', 'run out', 'retired out', 'obstructing the field', 'hit the ball twice', 'handled the ball', 'timed out')"
)

func roundTo2(value float64) float64 {
	return math.Round(value*100) / 100
}

func battingInningsSQL() string {
	return fmt.Sprintf(`
		SELECT
			d.match_id,
			d.innings,
			m.league,
			m.start_date,
			MAX(%[2]s) AS opposition,
			SUM(d.runs_off_bat) AS runs,
			COUNT(*) FILTER (WHERE d.wides = 0) AS balls,
			COUNT(*) FILTER (WHERE d.runs_off_bat = 4) AS fours,
			COUNT(*) FILTER (WHERE d.runs_off_bat = 6) AS sixes,
			EXISTS (
				SELECT 1
				FROM wpl_delivery w
				WHERE w.match_id = d.match_id
					AND w.innings = d.innings
					AND w.player_dismissed = $1
					AND w.wicket_type IN %[1]s
			) AS dismissed
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		WHERE d.striker = $1
			AND m.league = ANY(string_to_array($2, ','))
			AND d.innings <= 2
		GROUP BY d.match_id, d.innings, m.league, m.start_date`, battingDismissalTypesSQL, leagues.Default().TeamNameSQL("d.bowling_team"))
}

func bowlingInningsSQL() string {
	return fmt.Sprintf(`
		SELECT
			d.match_id,
			d.innings,
			m.league,
			m.start_date,
			MAX(%s) AS opposition,
			COALESCE(MAX(mi.balls_per_over), 6) AS balls_per_over,
			COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0) AS balls,
			SUM(d.runs_off_bat + d.wides + d.noballs) AS runs_conceded,
			COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL AND d.wicket_type IN %s) AS wickets,
			COUNT(*) FILTER (WHERE d.runs_off_bat = 0 AND d.wides = 0 AND d.noballs = 0) AS dot_balls
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		LEFT JOIN wpl_match_info mi ON mi.match_id = d.match_id
		WHERE d.bowler = $1
			AND m.league = ANY(string_to_array($2, ','))
			AND d.innings <= 2
		GROUP BY d.match_id, d.innings, m.league, m.start_date`, standardizedBattingTeamSQL(), bowlerWicketTypesSQL)
}

func (s *service) GetPlayerRecentForm(ctx context.Context, league, player string, limit int) (models.PlayerRecentForm, error) {
	form := models.PlayerRecentForm{
		Batting: []models.PlayerFormBatting{},
		Bowling: []models.PlayerFormBowling{},
	}

	battingRows, err := s.db.QueryContext(ctx, battingInningsSQL()+`
		ORDER BY m.start_date DESC, d.match_id DESC
		LIMIT $3`, player, league, limit)
	if err != nil {
		return form, fmt.Errorf("recent batting form: %w", err)
	}
	defer battingRows.Close()

	for battingRows.Next() {
		var (
			item      models.PlayerFormBatting
			innings   int
			date      time.Time
			dismissed bool
		)
		if err := battingRows.Scan(&item.MatchID, &innings, &item.League, &date, &item.Opposition, &item.Runs, &item.Balls, &item.Fours, &item.Sixes, &dismissed); err != nil {
			return form, fmt.Errorf("recent batting form: %w", err)
		}
		item.Date = date.Format("2006-01-02")
		item.NotOut = !dismissed
		if item.Balls > 0 {
			item.StrikeRate = roundTo2(float64(item.Runs) / float64(item.Balls) * 100)
		}
		form.Batting = append(form.Batting, item)
	}
	if err := battingRows.Err(); err != nil {
		return form, fmt.Errorf("recent batting form: %w", err)
	}

	bowlingRows, err := s.db.QueryContext(ctx, bowlingInningsSQL()+`
		ORDER BY m.start_date DESC, d.match_id DESC
		LIMIT $3`, player, league, limit)
	if err != nil {
		return form, fmt.Errorf("recent bowling form: %w", err)
	}
	defer bowlingRows.Close()

	for bowlingRows.Next() {
		var (
			item         models.PlayerFormBowling
			innings      int
			date         time.Time
			ballsPerOver int
			balls        int
		)
		if err := bowlingRows.Scan(&item.MatchID, &innings, &item.League, &date, &item.Opposition, &ballsPerOver, &balls, &item.RunsConceded, &item.Wickets, &item.DotBalls); err != nil {
			return form, fmt.Errorf("recent bowling form: %w", err)
		}
		item.Date = date.Format("2006-01-02")
		item.Overs = oversNotation(balls, ballsPerOver)
		form.Bowling = append(form.Bowling, item)
	}
	if err := bowlingRows.Err(); err != nil {
		return form, fmt.Errorf("recent bowling form: %w", err)
	}

	return form, nil
}

func (s *service) GetPlayerInningsSplits(ctx context.Context, league, player string) (models.PlayerInningsSplits, error) {
	splits := models.PlayerInningsSplits{
		Batting: []models.PlayerBattingSplit{},
		Bowling: []models.PlayerBowlingSplit{},
	}

	battingRows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		WITH innings AS (%s)
		SELECT
			innings,
			COUNT(*) AS played,
			SUM(runs) AS runs,
			SUM(balls) AS balls,
			COUNT(*) FILTER (WHERE dismissed) AS dismissals
		FROM innings
		GROUP BY innings
		ORDER BY innings`, battingInningsSQL()), player, league)
	if err != nil {
		return splits, fmt.Errorf("batting innings splits: %w", err)
	}
	defer battingRows.Close()

	for battingRows.Next() {
		var item models.PlayerBattingSplit
		if err := battingRows.Scan(&item.Innings, &item.Played, &item.Runs, &item.Balls, &item.Dismissals); err != nil {
			return splits, fmt.Errorf("batting innings splits: %w", err)
		}
		if item.Dismissals > 0 {
			item.Average = roundTo2(float64(item.Runs) / float64(item.Dismissals))
		}
		if item.Balls > 0 {
			item.StrikeRate = roundTo2(float64(item.Runs) / float64(item.Balls) * 100)
		}
		splits.Batting = append(splits.Batting, item)
	}
	if err := battingRows.Err(); err != nil {
		return splits, fmt.Errorf("batting innings splits: %w", err)
	}

	bowlingRows, err := s.db.QueryContext(ctx, fmt.Sprintf(`
		WITH innings AS (%s)
		SELECT
			innings,
			COUNT(*) AS played,
			SUM(balls) AS balls,
			SUM(runs_conceded) AS runs_conceded,
			SUM(wickets) AS wickets
		FROM innings
		GROUP BY innings
		ORDER BY innings`, bowlingInningsSQL()), player, league)
	if err != nil {
		return splits, fmt.Errorf("bowling innings splits: %w", err)
	}
	defer bowlingRows.Close()

	for bowlingRows.Next() {
		var item models.PlayerBowlingSplit
		if err := bowlingRows.Scan(&item.Innings, &item.Played, &item.Balls, &item.RunsConceded, &item.Wickets); err != nil {
			return splits, fmt.Errorf("bowling innings splits: %w", err)
		}
		if item.Balls > 0 {
			item.Economy = roundTo2(float64(item.RunsConceded) / (float64(item.Balls) / 6))
		}
		if item.Wickets > 0 {
			item.Average = roundTo2(float64(item.RunsConceded) / float64(item.Wickets))
		}
		splits.Bowling = append(splits.Bowling, item)
	}
	if err := bowlingRows.Err(); err != nil {
		return splits, fmt.Errorf("bowling innings splits: %w", err)
	}

	return splits, nil
}
