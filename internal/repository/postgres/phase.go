package postgres

import "fmt"

// Over numbers and phases.
//
// d.ball is "<over>.<ball>" with a 0-based over: "0.1" is the first ball of
// the first over. The Hundred uses the same numbering for its 5-ball sets
// (0-19), with wpl_match_info.balls_per_over = 5.
//
// Over-numbered stats ("over 16") count only matches of one over length,
// chosen by leagues.Registry.OverLength, so a T20 over and a Hundred set are
// never mixed. Phases work across formats: powerplay is the first 6 overs or
// 5 sets (36 or 25 balls), death is the last 5 overs or sets.

// overIndexSQL is a delivery's 0-based over (or set) number.
const overIndexSQL = "CAST(SPLIT_PART(d.ball, '.', 1) AS int)"

// matchInfoJoinSQL joins wpl_match_info as mi for its balls_per_over.
const matchInfoJoinSQL = "LEFT JOIN wpl_match_info mi ON mi.match_id = d.match_id"

// overLengthConditionSQL keeps deliveries from matches with the given over
// length; it needs mi joined.
func overLengthConditionSQL(ballsPerOver int) string {
	return fmt.Sprintf("COALESCE(mi.balls_per_over, 6) = %d", ballsPerOver)
}

// phaseSQL classifies a delivery as powerplay, middle or death; it needs mi
// joined.
func phaseSQL() string {
	return fmt.Sprintf(`CASE
		WHEN %[1]s < (CASE WHEN COALESCE(mi.balls_per_over, 6) = 5 THEN 5 ELSE 6 END) THEN 'powerplay'
		WHEN %[1]s >= 15 THEN 'death'
		ELSE 'middle'
	END`, overIndexSQL)
}

// phaseForOver returns the phase of a 1-based over (or set) number.
func phaseForOver(over, ballsPerOver int) string {
	powerplayOvers := 6
	if ballsPerOver == 5 {
		powerplayOvers = 5
	}
	switch {
	case over <= powerplayOvers:
		return "powerplay"
	case over <= 15:
		return "middle"
	default:
		return "death"
	}
}
