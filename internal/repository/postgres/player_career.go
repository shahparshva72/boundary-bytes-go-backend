package postgres

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func (s *service) GetPlayerCareer(ctx context.Context, league, player string) (models.PlayerCareer, error) {
	career := models.PlayerCareer{Seasons: []models.PlayerCareerRow{}}

	query := fmt.Sprintf(`
		WITH bat AS (%s),
		bowl AS (%s),
		bat_agg AS (
			SELECT
				league,
				season,
				MAX(start_date) AS last_played,
				COUNT(*) AS innings,
				COUNT(*) FILTER (WHERE NOT dismissed) AS not_outs,
				SUM(runs) AS runs,
				SUM(balls) AS balls,
				MAX(runs) AS highest_score,
				SUM(fours) AS fours,
				SUM(sixes) AS sixes,
				COUNT(*) FILTER (WHERE runs >= 50 AND runs < 100) AS fifties,
				COUNT(*) FILTER (WHERE runs >= 100) AS hundreds
			FROM bat
			GROUP BY league, season
		),
		bowl_agg AS (
			SELECT
				league,
				season,
				MAX(start_date) AS last_played,
				COUNT(*) AS innings,
				SUM(balls) AS balls,
				SUM(runs_conceded) AS runs_conceded,
				SUM(wickets) AS wickets,
				COUNT(*) FILTER (WHERE wickets >= 4 AND wickets < 5) AS four_wickets,
				COUNT(*) FILTER (WHERE wickets >= 5) AS five_wickets
			FROM bowl
			GROUP BY league, season
		)
		SELECT
			COALESCE(b.league, w.league),
			COALESCE(b.season, w.season),
			b.innings, b.not_outs, b.runs, b.balls, b.highest_score, b.fours, b.sixes, b.fifties, b.hundreds,
			w.innings, w.balls, w.runs_conceded, w.wickets, w.four_wickets, w.five_wickets
		FROM bat_agg b
		FULL JOIN bowl_agg w ON w.league = b.league AND w.season = b.season
		ORDER BY GREATEST(b.last_played, w.last_played) DESC`, battingInningsSQL(), bowlingInningsSQL())

	rows, err := s.db.QueryContext(ctx, query, player, league)
	if err != nil {
		return career, fmt.Errorf("player career: %w", err)
	}
	defer rows.Close()

	totalBatting := models.PlayerCompareBatting{}
	totalBowling := models.PlayerCompareBowling{}
	for rows.Next() {
		var (
			row                                                                        models.PlayerCareerRow
			batInnings, notOuts, runs, balls, highest, fours, sixes, fifties, hundreds sql.NullInt64
			bowlInnings, bowlBalls, conceded, wickets, fourWickets, fiveWickets        sql.NullInt64
		)
		if err := rows.Scan(&row.League, &row.Season,
			&batInnings, &notOuts, &runs, &balls, &highest, &fours, &sixes, &fifties, &hundreds,
			&bowlInnings, &bowlBalls, &conceded, &wickets, &fourWickets, &fiveWickets); err != nil {
			return career, fmt.Errorf("player career: %w", err)
		}

		if batInnings.Valid {
			batting := models.PlayerCompareBatting{
				Runs:         int(runs.Int64),
				BallsFaced:   int(balls.Int64),
				Innings:      int(batInnings.Int64),
				NotOuts:      int(notOuts.Int64),
				HighestScore: int(highest.Int64),
				Fours:        int(fours.Int64),
				Sixes:        int(sixes.Int64),
				Fifties:      int(fifties.Int64),
				Hundreds:     int(hundreds.Int64),
			}
			totalBatting.Runs += batting.Runs
			totalBatting.BallsFaced += batting.BallsFaced
			totalBatting.Innings += batting.Innings
			totalBatting.NotOuts += batting.NotOuts
			totalBatting.HighestScore = max(totalBatting.HighestScore, batting.HighestScore)
			totalBatting.Fours += batting.Fours
			totalBatting.Sixes += batting.Sixes
			totalBatting.Fifties += batting.Fifties
			totalBatting.Hundreds += batting.Hundreds
			finishCareerBatting(&batting)
			row.Batting = &batting
		}
		if bowlInnings.Valid {
			bowling := models.PlayerCompareBowling{
				Wickets:      int(wickets.Int64),
				BallsBowled:  int(bowlBalls.Int64),
				RunsConceded: int(conceded.Int64),
				Innings:      int(bowlInnings.Int64),
				FourWickets:  int(fourWickets.Int64),
				FiveWickets:  int(fiveWickets.Int64),
			}
			totalBowling.Wickets += bowling.Wickets
			totalBowling.BallsBowled += bowling.BallsBowled
			totalBowling.RunsConceded += bowling.RunsConceded
			totalBowling.Innings += bowling.Innings
			totalBowling.FourWickets += bowling.FourWickets
			totalBowling.FiveWickets += bowling.FiveWickets
			finishCareerBowling(&bowling)
			row.Bowling = &bowling
		}
		career.Seasons = append(career.Seasons, row)
	}
	if err := rows.Err(); err != nil {
		return career, fmt.Errorf("player career: %w", err)
	}

	if len(career.Seasons) > 0 {
		total := models.PlayerCareerRow{League: "All", Season: "Career"}
		if totalBatting.Innings > 0 {
			finishCareerBatting(&totalBatting)
			total.Batting = &totalBatting
		}
		if totalBowling.Innings > 0 {
			finishCareerBowling(&totalBowling)
			total.Bowling = &totalBowling
		}
		career.Total = &total
	}

	return career, nil
}

func finishCareerBatting(batting *models.PlayerCompareBatting) {
	if batting.BallsFaced > 0 {
		batting.StrikeRate = roundTo2(float64(batting.Runs) / float64(batting.BallsFaced) * 100)
	}
	if dismissals := batting.Innings - batting.NotOuts; dismissals > 0 {
		batting.Average = roundTo2(float64(batting.Runs) / float64(dismissals))
	} else {
		batting.Average = float64(batting.Runs)
	}
}

func finishCareerBowling(bowling *models.PlayerCompareBowling) {
	if bowling.BallsBowled > 0 {
		bowling.Economy = roundTo2(float64(bowling.RunsConceded) / (float64(bowling.BallsBowled) / 6))
	}
	if bowling.Wickets > 0 {
		bowling.Average = roundTo2(float64(bowling.RunsConceded) / float64(bowling.Wickets))
		bowling.StrikeRate = roundTo2(float64(bowling.BallsBowled) / float64(bowling.Wickets))
	}
}
