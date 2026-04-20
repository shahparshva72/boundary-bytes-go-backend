package models

import "time"

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
	League     string                      `json:"league"`
	Pagination Pagination                  `json:"pagination"`
	Metadata   LeadingWicketTakersMetadata `json:"metadata"`
	Data       []WicketTaker               `json:"data"`
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
	Data       []RunScorer               `json:"data"`
	League     string                    `json:"league"`
	Pagination Pagination                `json:"pagination"`
	Metadata   LeadingRunScorersMetadata `json:"metadata"`
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

type NewsAPIResponse struct {
	Success bool        `json:"success"`
	Data    NewsAPIData `json:"data"`
}

type NewsAPIData struct {
	Title       string            `json:"title"`
	Description string            `json:"description"`
	Link        string            `json:"link"`
	Items       []RSSItemResponse `json:"items"`
}

type RSSItemResponse struct {
	Title          *string    `json:"title"`
	Link           *string    `json:"link"`
	PubDate        *string    `json:"pubDate"`
	ContentSnippet *string    `json:"contentSnippet"`
	Content        *string    `json:"content"`
	GUID           *string    `json:"guid"`
	Enclosure      *Enclosure `json:"enclosure,omitempty"`
	Image          *string    `json:"image,omitempty"`
}

type Enclosure struct {
	URL *string `json:"url"`
}

type SeasonsResponse struct {
	Seasons  []string        `json:"seasons"`
	League   string          `json:"league"`
	Metadata SeasonsMetadata `json:"metadata"`
}

type SeasonsMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalSeasons     int      `json:"totalSeasons"`
}

type MatchListItem struct {
	ID     int    `json:"id"`
	League string `json:"league"`
	Teams  string `json:"teams"`
	Venue  string `json:"venue"`
	Date   string `json:"date"`
	Season string `json:"season"`
}

type MatchListResponse struct {
	Data     []MatchListItem   `json:"data"`
	League   string            `json:"league"`
	Metadata MatchListMetadata `json:"metadata"`
}

type MatchListMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalRecords     int      `json:"totalRecords"`
}

type MatchCard struct {
	ID            int       `json:"id"`
	League        string    `json:"league"`
	Season        string    `json:"season"`
	StartDate     time.Time `json:"startDate"`
	Venue         string    `json:"venue"`
	Team1         string    `json:"team1"`
	Team2         string    `json:"team2"`
	Innings1Score string    `json:"innings1Score"`
	Innings2Score string    `json:"innings2Score"`
	Result        string    `json:"result"`
}

type MatchesResponse struct {
	Matches    []MatchCard    `json:"matches"`
	League     string         `json:"league"`
	Pagination Pagination     `json:"pagination"`
	Seasons    []string       `json:"seasons"`
	Metadata   MatchesMetadata `json:"metadata"`
}

type MatchesMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalRecords     int      `json:"totalRecords"`
}

type TeamWinsItem struct {
	Team              string `json:"team"`
	MatchesPlayed     int    `json:"matchesPlayed"`
	Wins              int    `json:"wins"`
	Losses            int    `json:"losses"`
	WinsBattingFirst  int    `json:"winsBattingFirst"`
	WinsBattingSecond int    `json:"winsBattingSecond"`
}

type TeamWinsResponse struct {
	Data     []TeamWinsItem   `json:"data"`
	League   string           `json:"league"`
	Metadata TeamWinsMetadata `json:"metadata"`
}

type TeamWinsMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalTeams       int      `json:"totalTeams"`
}

type TeamAverageItem struct {
	Team            string  `json:"team"`
	TotalInnings    int     `json:"totalInnings"`
	TotalRuns       int     `json:"totalRuns"`
	TotalBalls      int     `json:"totalBalls"`
	TotalDismissals int     `json:"totalDismissals"`
	BattingAverage  float64 `json:"battingAverage"`
	StrikeRate      float64 `json:"strikeRate"`
	HighestScore    int     `json:"highestScore"`
	LowestScore     int     `json:"lowestScore"`
}

type TeamAveragesResponse struct {
	Data     []TeamAverageItem   `json:"data"`
	League   string              `json:"league"`
	Metadata TeamAveragesMetadata `json:"metadata"`
}

type TeamAveragesMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalTeams       int      `json:"totalTeams"`
}

type BowlingWicketTypesItem struct {
	Player       string                     `json:"player"`
	TotalWickets int                        `json:"totalWickets"`
	WicketTypes  BowlingWicketTypeBreakdown `json:"wicketTypes"`
	Matches      int                        `json:"matches"`
}

type BowlingWicketTypeBreakdown struct {
	Caught          int `json:"caught"`
	Bowled          int `json:"bowled"`
	Lbw             int `json:"lbw"`
	Stumped         int `json:"stumped"`
	CaughtAndBowled int `json:"caughtAndBowled"`
	HitWicket       int `json:"hitWicket"`
}

type BowlingWicketTypesResponse struct {
	Data       []BowlingWicketTypesItem   `json:"data"`
	League     string                     `json:"league"`
	Pagination Pagination                 `json:"pagination"`
	Metadata   BowlingWicketTypesMetadata `json:"metadata"`
}

type BowlingWicketTypesMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalRecords     int      `json:"totalRecords"`
}

type MultiMatchupItem struct {
	Opponent     string  `json:"opponent"`
	RunsScored   int     `json:"runsScored"`
	BallsFaced   int     `json:"ballsFaced"`
	Dismissals   int     `json:"dismissals"`
	StrikeRate   float64 `json:"strikeRate"`
	EconomyRate  float64 `json:"economyRate"`
	Average      float64 `json:"average"`
	Fours        int     `json:"fours"`
	Sixes        int     `json:"sixes"`
	DotBalls     int     `json:"dotBalls"`
}

type MultiMatchupCombined struct {
	RunsScored  int     `json:"runsScored"`
	BallsFaced  int     `json:"ballsFaced"`
	Dismissals  int     `json:"dismissals"`
	StrikeRate  float64 `json:"strikeRate"`
	EconomyRate float64 `json:"economyRate"`
	Average     float64 `json:"average"`
	Fours       int     `json:"fours"`
	Sixes       int     `json:"sixes"`
	DotBalls    int     `json:"dotBalls"`
}

type MultiMatchupResponse struct {
	Data      []MultiMatchupItem   `json:"data"`
	Combined  MultiMatchupCombined `json:"combined"`
	League    string               `json:"league"`
	Player    string               `json:"player"`
	Mode      string               `json:"mode"`
	Opponents []string             `json:"opponents"`
	Metadata  MultiMatchupMetadata `json:"metadata"`
}

type MultiMatchupMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	ResultCount      int      `json:"resultCount"`
}

type PlayerProgressionPoint struct {
	Over        int      `json:"over"`
	Phase       string   `json:"phase"`
	Runs        int      `json:"runs"`
	Balls       int      `json:"balls"`
	Dismissals  int      `json:"dismissals"`
	StrikeRate  float64  `json:"strikeRate"`
	Average     *float64 `json:"average"`
}

type PlayerProgressionResponse struct {
	Data     []PlayerProgressionPoint   `json:"data"`
	Player   string                     `json:"player"`
	League   string                     `json:"league"`
	Innings  *string                    `json:"innings"`
	Metadata PlayerProgressionMetadata  `json:"metadata"`
}

type PlayerProgressionMetadata struct {
	TotalInnings     int      `json:"totalInnings"`
	TotalMatches     int      `json:"totalMatches"`
	TotalDeliveries  int      `json:"totalDeliveries"`
	AvailableLeagues []string `json:"availableLeagues"`
}

type AdvancedStatsBatterData struct {
	RunsScored int     `json:"runsScored"`
	BallsFaced int     `json:"ballsFaced"`
	StrikeRate float64 `json:"strikeRate"`
	Average    float64 `json:"average"`
	Fours      int     `json:"fours"`
	Sixes      int     `json:"sixes"`
	Dismissals int     `json:"dismissals"`
}

type AdvancedStatsBowlerData struct {
	RunsConceded int     `json:"runsConceded"`
	BallsBowled  int     `json:"ballsBowled"`
	Overs        float64 `json:"overs"`
	Wickets      int     `json:"wickets"`
	EconomyRate  float64 `json:"economyRate"`
	Average      float64 `json:"average"`
	StrikeRate   float64 `json:"strikeRate"`
	Dots         int     `json:"dots"`
	Wides        int     `json:"wides"`
	Noballs      int     `json:"noballs"`
}

type AdvancedStatsResponse struct {
	Data       interface{}            `json:"data"`
	League     string                 `json:"league"`
	Player     string                 `json:"player"`
	PlayerType string                 `json:"playerType"`
	Overs      []int                  `json:"overs"`
	Metadata   AdvancedStatsMetadata  `json:"metadata"`
}

type AdvancedStatsMetadata struct {
	AvailableLeagues   []string `json:"availableLeagues"`
	DeliveriesAnalyzed int      `json:"deliveriesAnalyzed"`
}

type FallOfWicketsResponse struct {
	MatchInfo FallOfWicketsMatchInfo `json:"matchInfo"`
	Innings   []FallOfWicketsInnings `json:"innings"`
	Metadata  FallOfWicketsMetadata  `json:"metadata"`
}

type FallOfWicketsMatchInfo struct {
	ID     int      `json:"id"`
	League string   `json:"league"`
	Teams  []string `json:"teams"`
	Venue  string   `json:"venue"`
	Date   string   `json:"date"`
	Season string   `json:"season"`
}

type FallOfWicketsInnings struct {
	InningsNumber int                   `json:"inningsNumber"`
	BattingTeam   string                `json:"battingTeam"`
	Wickets       []FallOfWicketsEntry  `json:"wickets"`
}

type FallOfWicketsEntry struct {
	WicketNumber  int    `json:"wicketNumber"`
	Over          string `json:"over"`
	RunsAtFall    int    `json:"runsAtFall"`
	BatsmanOut    string `json:"batsmanOut"`
	DismissalType string `json:"dismissalType"`
	Bowler        string `json:"bowler"`
}

type FallOfWicketsMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalWickets     int      `json:"totalWickets"`
}
