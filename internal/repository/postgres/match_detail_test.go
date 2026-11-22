package postgres

import (
	"database/sql"
	"testing"
)

func ball(over string, striker, nonStriker, bowler string, runs int) matchDelivery {
	return matchDelivery{innings: 1, ball: over, battingTeam: "A", bowlingTeam: "B", striker: striker, nonStriker: nonStriker, bowler: bowler, runsOffBat: runs}
}

func TestBuildMatchInnings(t *testing.T) {
	wide := ball("0.3", "X", "Y", "P", 0)
	wide.wides, wide.extras = 1, 1
	noball := ball("0.4", "X", "Y", "P", 4)
	noball.noballs, noball.extras = 1, 1
	wicket := ball("0.5", "X", "Y", "P", 0)
	wicket.wicketType = sql.NullString{String: "bowled", Valid: true}
	wicket.playerDismissed = sql.NullString{String: "X", Valid: true}
	runOut := ball("15.1", "Z", "Y", "Q", 1)
	runOut.wicketType = sql.NullString{String: "run out", Valid: true}
	runOut.playerDismissed = sql.NullString{String: "Y", Valid: true}

	innings := buildMatchInnings(1, []matchDelivery{
		ball("0.1", "X", "Y", "P", 1),
		ball("0.2", "Y", "X", "P", 6),
		wide,
		noball,
		wicket,
		ball("0.6", "Z", "Y", "P", 0),
		runOut,
	}, 6)

	if innings.Runs != 14 || innings.Wickets != 2 || innings.Balls != 5 {
		t.Fatalf("runs/wickets/balls = %d/%d/%d, want 14/2/5", innings.Runs, innings.Wickets, innings.Balls)
	}
	if innings.Extras.Total != 2 || innings.Extras.Wides != 1 || innings.Extras.Noballs != 1 {
		t.Fatalf("extras = %+v", innings.Extras)
	}

	x := innings.Batting[0]
	if x.Player != "X" || x.Runs != 5 || x.Balls != 3 || x.Fours != 1 || x.NotOut || x.Dismissal != "b P" {
		t.Fatalf("batter X = %+v", x)
	}
	y := innings.Batting[1]
	if y.Runs != 6 || y.Balls != 1 || y.NotOut || y.Dismissal != "run out" {
		t.Fatalf("batter Y = %+v", y)
	}
	if z := innings.Batting[2]; !z.NotOut || z.Runs != 1 || z.Balls != 2 {
		t.Fatalf("batter Z = %+v", z)
	}

	p := innings.Bowling[0]
	if p.Balls != 4 || p.RunsConceded != 13 || p.Wickets != 1 || p.DotBalls != 2 || p.Overs != 0.4 {
		t.Fatalf("bowler P = %+v", p)
	}
	if q := innings.Bowling[1]; q.Wickets != 0 || q.RunsConceded != 1 {
		t.Fatalf("run out credited to bowler: %+v", q)
	}

	if len(innings.Partnerships) != 2 {
		t.Fatalf("partnerships = %+v", innings.Partnerships)
	}
	first := innings.Partnerships[0]
	if first.Runs != 13 || first.Balls != 3 || first.Unbroken || first.Batter1Runs != 5 || first.Batter2Runs != 6 {
		t.Fatalf("first partnership = %+v", first)
	}
	if second := innings.Partnerships[1]; second.Wicket != 2 || second.Runs != 1 || second.Unbroken {
		t.Fatalf("second partnership = %+v", second)
	}

	if len(innings.FallOfWickets) != 2 || innings.FallOfWickets[0].RunsAtFall != 13 || innings.FallOfWickets[1].WicketNumber != 2 || innings.FallOfWickets[1].RunsAtFall != 14 {
		t.Fatalf("fall of wickets = %+v", innings.FallOfWickets)
	}
	if len(innings.OverByOver) != 2 || innings.OverByOver[0].Runs != 13 || innings.OverByOver[1].Over != 16 || innings.OverByOver[1].CumulativeRuns != 14 {
		t.Fatalf("overs = %+v", innings.OverByOver)
	}
	if len(innings.Phases) != 2 || innings.Phases[0].Phase != "powerplay" || innings.Phases[1].Phase != "death" {
		t.Fatalf("phases = %+v", innings.Phases)
	}
}
