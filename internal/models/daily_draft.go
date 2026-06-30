package models

type DailyDraftScoreRecord struct {
	ID           string  `json:"id"`
	DeviceID     string  `json:"deviceId"`
	League       string  `json:"league"`
	PlayDate     string  `json:"date"`
	Score        float64 `json:"score"`
	OptimalScore float64 `json:"optimalScore"`
	Lineup       string  `json:"lineup"`
}

type DailyDraftLeaderboardEntry struct {
	Rank  int     `json:"rank"`
	Score float64 `json:"score"`
	IsYou bool    `json:"isYou"`
}

type DailyDraftLeaderboardResponse struct {
	League       string                       `json:"league"`
	Date         string                       `json:"date"`
	TotalPlayers int                          `json:"totalPlayers"`
	YourRank     *int                         `json:"yourRank"`
	YourScore    *float64                     `json:"yourScore"`
	TopScores    []DailyDraftLeaderboardEntry `json:"topScores"`
}

type SubmitDailyDraftScoreParams struct {
	DeviceID     string
	League       string
	Date         string
	Score        float64
	OptimalScore float64
	Lineup       []string
}
