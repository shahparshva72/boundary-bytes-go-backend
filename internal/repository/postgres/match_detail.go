package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

var (
	bowlerWicketTypes = map[string]bool{
		"caught": true, "bowled": true, "lbw": true, "stumped": true, "caught and bowled": true, "hit wicket": true,
	}
	battingDismissalTypes = map[string]bool{
		"caught": true, "bowled": true, "lbw": true, "stumped": true, "caught and bowled": true, "hit wicket": true,
		"run out": true, "retired out": true, "obstructing the field": true, "hit the ball twice": true,
		"handled the ball": true, "timed out": true,
	}
)

type matchDelivery struct {
	innings              int
	ball                 string
	battingTeam          string
	bowlingTeam          string
	striker              string
	nonStriker           string
	bowler               string
	runsOffBat           int
	extras               int
	wides                int
	noballs              int
	byes                 int
	legbyes              int
	penalty              int
	wicketType           sql.NullString
	playerDismissed      sql.NullString
	otherPlayerDismissed sql.NullString
	otherWicketType      sql.NullString
}

func (s *service) GetMatchDetail(ctx context.Context, league string, matchID int) (*models.MatchDetailResponse, error) {
	var (
		info          models.MatchDetailInfo
		date          sql.NullTime
		playerOfMatch sql.NullString
		winner        sql.NullString
		winnerRuns    sql.NullInt64
		winnerWickets sql.NullInt64
	)
	err := s.db.QueryRowContext(ctx, `
		SELECT mi.match_id, mi.league, mi.season, mi.date, mi.venue, mi.city, mi.toss_winner, mi.toss_decision,
			mi.player_of_match, mi.winner, mi.winner_runs, mi.winner_wickets, COALESCE(mi.balls_per_over, 6)
		FROM wpl_match_info mi
		WHERE mi.match_id = $1 AND mi.league = ANY(string_to_array($2, ','))`, matchID, league).Scan(
		&info.ID, &info.League, &info.Season, &date, &info.Venue, &info.City, &info.TossWinner, &info.TossDecision,
		&playerOfMatch, &winner, &winnerRuns, &winnerWickets, &info.BallsPerOver)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("match info: %w", err)
	}
	if date.Valid {
		info.Date = date.Time.Format("2006-01-02")
	}
	info.PlayerOfMatch = playerOfMatch.String

	rows, err := s.db.QueryContext(ctx, `
		SELECT innings, ball, batting_team, bowling_team, striker, non_striker, bowler,
			runs_off_bat, extras, wides, noballs, byes, legbyes, penalty,
			wicket_type, player_dismissed, other_player_dismissed, other_wicket_type
		FROM wpl_delivery
		WHERE match_id = $1 AND innings <= 2
		ORDER BY innings, CAST(SPLIT_PART(ball, '.', 1) AS int), CAST(SPLIT_PART(ball, '.', 2) AS int), id`, matchID)
	if err != nil {
		return nil, fmt.Errorf("match deliveries: %w", err)
	}
	defer rows.Close()

	byInnings := map[int][]matchDelivery{}
	for rows.Next() {
		var d matchDelivery
		if err := rows.Scan(&d.innings, &d.ball, &d.battingTeam, &d.bowlingTeam, &d.striker, &d.nonStriker, &d.bowler,
			&d.runsOffBat, &d.extras, &d.wides, &d.noballs, &d.byes, &d.legbyes, &d.penalty,
			&d.wicketType, &d.playerDismissed, &d.otherPlayerDismissed, &d.otherWicketType); err != nil {
			return nil, fmt.Errorf("match deliveries: %w", err)
		}
		byInnings[d.innings] = append(byInnings[d.innings], d)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("match deliveries: %w", err)
	}

	response := &models.MatchDetailResponse{Innings: []models.MatchInnings{}}
	for number := 1; number <= 2; number++ {
		deliveries := byInnings[number]
		if len(deliveries) == 0 {
			continue
		}
		response.Innings = append(response.Innings, buildMatchInnings(number, deliveries, info.BallsPerOver))
	}

	var team1, team2 sql.NullString
	var score1, score2, wickets2 int
	for _, innings := range response.Innings {
		if innings.Innings == 1 {
			team1 = sql.NullString{String: innings.BattingTeam, Valid: true}
			team2 = sql.NullString{String: innings.BowlingTeam, Valid: true}
			score1 = innings.Runs
			info.Teams = []string{innings.BattingTeam, innings.BowlingTeam}
		} else {
			score2, wickets2 = innings.Runs, innings.Wickets
		}
	}
	if len(response.Innings) > 0 {
		info.Result = formatMatchResult(winner, winnerRuns, winnerWickets, team1, team2, score1, score2, wickets2)
	}
	if info.Teams == nil {
		info.Teams = []string{}
	}
	response.Match = info

	return response, nil
}

func buildMatchInnings(number int, deliveries []matchDelivery, ballsPerOver int) models.MatchInnings {
	innings := models.MatchInnings{
		Innings:       number,
		BattingTeam:   deliveries[0].battingTeam,
		BowlingTeam:   deliveries[0].bowlingTeam,
		Batting:       []models.MatchBattingRow{},
		Bowling:       []models.MatchBowlingRow{},
		Partnerships:  []models.MatchPartnership{},
		OverByOver:    []models.MatchOver{},
		Phases:        []models.MatchPhaseSummary{},
		FallOfWickets: []models.FallOfWicketsEntry{},
	}

	batters := map[string]*models.MatchBattingRow{}
	batterOrder := []string{}
	batter := func(name string) *models.MatchBattingRow {
		if row, ok := batters[name]; ok {
			return row
		}
		row := &models.MatchBattingRow{Player: name, Dismissal: "not out", NotOut: true}
		batters[name] = row
		batterOrder = append(batterOrder, name)
		return row
	}
	bowlers := map[string]*models.MatchBowlingRow{}
	bowlerOrder := []string{}
	overs := map[int]*models.MatchOver{}
	overOrder := []int{}
	phases := map[string]*models.MatchPhaseSummary{}
	rateBalls := 0
	var partnership *models.MatchPartnership

	for _, d := range deliveries {
		total := d.runsOffBat + d.extras
		legal := d.wides == 0 && d.noballs == 0
		faced := d.wides == 0

		striker := batter(d.striker)
		batter(d.nonStriker)
		striker.Runs += d.runsOffBat
		if faced {
			striker.Balls++
			rateBalls++
		}
		if d.runsOffBat == 4 {
			striker.Fours++
		}
		if d.runsOffBat == 6 {
			striker.Sixes++
		}

		bowler, ok := bowlers[d.bowler]
		if !ok {
			bowler = &models.MatchBowlingRow{Player: d.bowler}
			bowlers[d.bowler] = bowler
			bowlerOrder = append(bowlerOrder, d.bowler)
		}
		bowler.RunsConceded += d.runsOffBat + d.wides + d.noballs
		bowler.Wides += d.wides
		bowler.Noballs += d.noballs
		if legal {
			bowler.Balls++
			innings.Balls++
			if d.runsOffBat == 0 {
				bowler.DotBalls++
			}
		}

		innings.Runs += total
		innings.Extras.Total += d.extras
		innings.Extras.Wides += d.wides
		innings.Extras.Noballs += d.noballs
		innings.Extras.Byes += d.byes
		innings.Extras.Legbyes += d.legbyes
		innings.Extras.Penalty += d.penalty

		wicketsOnBall := 0
		fall := func(player, wicketType string) {
			wicketsOnBall++
			innings.FallOfWickets = append(innings.FallOfWickets, models.FallOfWicketsEntry{
				WicketNumber:  innings.Wickets + wicketsOnBall,
				Over:          d.ball,
				RunsAtFall:    innings.Runs,
				BatsmanOut:    player,
				DismissalType: wicketType,
				Bowler:        d.bowler,
			})
		}
		if d.playerDismissed.Valid && d.playerDismissed.String != "" {
			wicketType := d.wicketType.String
			fall(d.playerDismissed.String, wicketType)
			if bowlerWicketTypes[wicketType] {
				bowler.Wickets++
			}
			dismissed := batter(d.playerDismissed.String)
			dismissed.Dismissal = dismissalText(wicketType, d.bowler)
			dismissed.NotOut = !battingDismissalTypes[wicketType]
		}
		if d.otherPlayerDismissed.Valid && d.otherPlayerDismissed.String != "" {
			fall(d.otherPlayerDismissed.String, d.otherWicketType.String)
		}
		innings.Wickets += wicketsOnBall

		overIndex, _ := strconv.Atoi(strings.SplitN(d.ball, ".", 2)[0])
		over, ok := overs[overIndex]
		if !ok {
			over = &models.MatchOver{Over: overIndex + 1}
			overs[overIndex] = over
			overOrder = append(overOrder, overIndex)
		}
		over.Runs += total
		over.Wickets += wicketsOnBall

		phaseName := phaseForOver(overIndex+1, ballsPerOver)
		phase, ok := phases[phaseName]
		if !ok {
			phase = &models.MatchPhaseSummary{Phase: phaseName}
			phases[phaseName] = phase
		}
		phase.Runs += total
		phase.Wickets += wicketsOnBall
		if faced {
			phase.Balls++
		}

		if partnership == nil || !samePair(partnership, d.striker, d.nonStriker) {
			if partnership != nil {
				innings.Partnerships = append(innings.Partnerships, *partnership)
			}
			partnership = &models.MatchPartnership{
				Wicket:   len(innings.Partnerships) + 1,
				Batter1:  d.striker,
				Batter2:  d.nonStriker,
				Unbroken: true,
			}
		}
		partnership.Runs += total
		if legal {
			partnership.Balls++
		}
		if d.striker == partnership.Batter1 {
			partnership.Batter1Runs += d.runsOffBat
			if faced {
				partnership.Batter1Balls++
			}
		} else {
			partnership.Batter2Runs += d.runsOffBat
			if faced {
				partnership.Batter2Balls++
			}
		}
		if wicketsOnBall > 0 {
			partnership.Unbroken = false
		}
	}
	if partnership != nil {
		innings.Partnerships = append(innings.Partnerships, *partnership)
	}

	for _, name := range batterOrder {
		row := batters[name]
		if row.Balls > 0 {
			row.StrikeRate = roundTo2(float64(row.Runs) / float64(row.Balls) * 100)
		}
		innings.Batting = append(innings.Batting, *row)
	}
	for _, name := range bowlerOrder {
		row := bowlers[name]
		row.Overs = oversNotation(row.Balls, ballsPerOver)
		if row.Balls > 0 {
			row.Economy = roundTo2(float64(row.RunsConceded) / (float64(row.Balls) / 6))
		}
		innings.Bowling = append(innings.Bowling, *row)
	}
	cumulativeRuns, cumulativeWickets := 0, 0
	for _, index := range overOrder {
		over := overs[index]
		cumulativeRuns += over.Runs
		cumulativeWickets += over.Wickets
		over.CumulativeRuns = cumulativeRuns
		over.CumulativeWkts = cumulativeWickets
		innings.OverByOver = append(innings.OverByOver, *over)
	}
	for _, name := range []string{"powerplay", "middle", "death"} {
		if phase, ok := phases[name]; ok {
			if phase.Balls > 0 {
				phase.RunRate = roundTo2(float64(phase.Runs) / (float64(phase.Balls) / 6))
			}
			innings.Phases = append(innings.Phases, *phase)
		}
	}
	innings.Overs = oversNotation(innings.Balls, ballsPerOver)
	if rateBalls > 0 {
		innings.RunRate = roundTo2(float64(innings.Runs) / (float64(rateBalls) / 6))
	}

	return innings
}

func samePair(partnership *models.MatchPartnership, striker, nonStriker string) bool {
	return (partnership.Batter1 == striker && partnership.Batter2 == nonStriker) ||
		(partnership.Batter1 == nonStriker && partnership.Batter2 == striker)
}

func dismissalText(wicketType, bowler string) string {
	switch wicketType {
	case "bowled":
		return "b " + bowler
	case "caught":
		return "c b " + bowler
	case "caught and bowled":
		return "c & b " + bowler
	case "lbw", "stumped", "hit wicket":
		return wicketType + " b " + bowler
	default:
		return wicketType
	}
}
