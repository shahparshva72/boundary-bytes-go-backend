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
