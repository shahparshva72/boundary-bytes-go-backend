package models

type PlayerListResponse struct {
	Data     []string           `json:"data"`
	League   string             `json:"league"`
	Metadata PlayerListMetadata `json:"metadata"`
}

type PlayerListMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalRecords     int      `json:"totalRecords"`
}

type MatchupResponse struct {
	Data     MatchupData     `json:"data"`
	League   string          `json:"league"`
	Batter   string          `json:"batter"`
	Bowler   string          `json:"bowler"`
	Metadata MatchupMetadata `json:"metadata"`
}

type MatchupData struct {
	RunsScored int     `json:"runsScored"`
	BallsFaced int     `json:"ballsFaced"`
	Dismissals int     `json:"dismissals"`
	StrikeRate float64 `json:"strikeRate"`
	Average    float64 `json:"average"`
}

type MatchupMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	MatchupExists    bool     `json:"matchupExists"`
}

type LeadingWicketTakersResponse struct {
	League     string                    `json:"league"`
	Pagination Pagination                `json:"pagination"`
	Metadata   LeadingWicketTakersMetadata `json:"metadata"`
	Data       []WicketTaker             `json:"data"`
}

type Pagination struct {
	Total       int `json:"total"`
	Pages       int `json:"pages"`
	CurrentPage int `json:"currentPage"`
	Limit       int `json:"limit"`
}

type LeadingWicketTakersMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalRecords     int      `json:"totalRecords"`
}

type WicketTaker struct {
	Player       string  `json:"player"`
	Wickets      int     `json:"wickets"`
	RunsConceded int     `json:"runsConceded"`
	Average      float64 `json:"average"`
	BallsBowled  int     `json:"ballsBowled"`
	Economy      float64 `json:"economy"`
	Matches      int     `json:"matches"`
}

type LeadingRunScorersResponse struct {
	Data       []RunScorer                 `json:"data"`
	League     string                      `json:"league"`
	Pagination Pagination                  `json:"pagination"`
	Metadata   LeadingRunScorersMetadata   `json:"metadata"`
}

type LeadingRunScorersMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalRecords     int      `json:"totalRecords"`
}

type RunScorer struct {
	Player            string  `json:"player"`
	Runs              int     `json:"runs"`
	BallsFaced        int     `json:"ballsFaced"`
	StrikeRate        float64 `json:"strikeRate"`
	Matches           int     `json:"matches"`
	Fours             int     `json:"fours"`
	Sixes             int     `json:"sixes"`
	DotBallPercentage float64 `json:"dotBallPercentage"`
}
