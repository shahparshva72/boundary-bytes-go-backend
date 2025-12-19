package models

import "time"

type Match struct {
	ID        int       `json:"id"`
	League    string    `json:"league"`
	Season    string    `json:"season"`
	StartDate time.Time `json:"start_date"`
	Venue     string    `json:"venue"`
}

type Delivery struct {
	ID                   int     `json:"id"`
	MatchID              int     `json:"match_id"`
	Innings              int     `json:"innings"`
	Ball                 string  `json:"ball"`
	BattingTeam          string  `json:"batting_team"`
	BowlingTeam          string  `json:"bowling_team"`
	Striker              string  `json:"striker"`
	NonStriker           string  `json:"non_striker"`
	Bowler               string  `json:"bowler"`
	RunsOffBat           int     `json:"runs_off_bat"`
	Extras               int     `json:"extras"`
	Wides                int     `json:"wides"`
	Noballs              int     `json:"noballs"`
	Byes                 int     `json:"byes"`
	Legbyes              int     `json:"legbyes"`
	Penalty              int     `json:"penalty"`
	WicketType           *string `json:"wicket_type,omitempty"`
	PlayerDismissed      *string `json:"player_dismissed,omitempty"`
	OtherWicketType      *string `json:"other_wicket_type,omitempty"`
	OtherPlayerDismissed *string `json:"other_player_dismissed,omitempty"`
}

type MatchInfo struct {
	ID            int       `json:"id"`
	League        string    `json:"league"`
	Version       string    `json:"version"`
	BallsPerOver  int       `json:"balls_per_over"`
	Gender        string    `json:"gender"`
	Season        string    `json:"season"`
	Date          time.Time `json:"date"`
	Event         string    `json:"event"`
	MatchNumber   int       `json:"match_number"`
	Venue         string    `json:"venue"`
	City          string    `json:"city"`
	TossWinner    string    `json:"toss_winner"`
	TossDecision  string    `json:"toss_decision"`
	PlayerOfMatch *string   `json:"player_of_match,omitempty"`
	Winner        *string   `json:"winner,omitempty"`
	WinnerRuns    *int      `json:"winner_runs,omitempty"`
	WinnerWickets *int      `json:"winner_wickets,omitempty"`
}

type Team struct {
	ID       int    `json:"id"`
	MatchID  int    `json:"match_id"`
	TeamName string `json:"team_name"`
}

type Player struct {
	ID         int    `json:"id"`
	MatchID    int    `json:"match_id"`
	TeamName   string `json:"team_name"`
	PlayerName string `json:"player_name"`
}

type Official struct {
	ID           int    `json:"id"`
	MatchID      int    `json:"match_id"`
	OfficialType string `json:"official_type"`
	OfficialName string `json:"official_name"`
}

type PersonRegistry struct {
	ID         int    `json:"id"`
	MatchID    int    `json:"match_id"`
	PersonName string `json:"person_name"`
	RegistryID string `json:"registry_id"`
}

type LeagueConfig struct {
	League       string `json:"league"`
	CSVDirectory string `json:"csv_directory"`
}

type AIChatRequest struct {
	ID                string    `json:"id"`
	Question          string    `json:"question"`
	SanitizedQuestion string    `json:"sanitized_question"`
	League            string    `json:"league"`
	GeneratedSQL      string    `json:"generated_sql"`
	RowCount          int       `json:"row_count"`
	ExecutionTimeMS   int       `json:"execution_time_ms"`
	Success           bool      `json:"success"`
	ErrorCode         string    `json:"error_code"`
	ErrorMessage      string    `json:"error_message"`
	IsAccurate        bool      `json:"is_accurate"`
	FeedbackNote      string    `json:"feedback_note"`
	FeedbackAt        time.Time `json:"feedback_at"`
	CreatedAt         time.Time `json:"created_at"`
}
