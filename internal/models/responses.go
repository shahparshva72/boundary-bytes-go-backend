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

type PlayerSlugEntry struct {
	Slug       string   `json:"slug"`
	PlayerName string   `json:"playerName"`
	Leagues    []string `json:"leagues"`
}

type PlayerProfileResponse struct {
	Slug        string                     `json:"slug"`
	Name        string                     `json:"name"`
	Bio         *PlayerProfileBio          `json:"bio,omitempty"`
	LeagueStats []PlayerProfileLeagueStats `json:"leagueStats"`
	Metadata    PlayerProfileMetadata      `json:"metadata"`
}

type PlayerProfileBio struct {
	FullName          *string `json:"fullName,omitempty"`
	BattingHand       *string `json:"battingHand,omitempty"`
	BowlingHand       *string `json:"bowlingHand,omitempty"`
	BowlingType       *string `json:"bowlingType,omitempty"`
	PlayingRole       *string `json:"playingRole,omitempty"`
	PlayingRoleDetail *string `json:"playingRoleDetail,omitempty"`
}

type PlayerProfileLeagueStats struct {
	League  string                `json:"league"`
	Batting *PlayerCompareBatting `json:"batting,omitempty"`
	Bowling *PlayerCompareBowling `json:"bowling,omitempty"`
}

type PlayerProfileMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
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
	Fifties           int     `json:"fifties"`
	Hundreds          int     `json:"hundreds"`
}

type LatestMatchDateResponse struct {
	League     string  `json:"league"`
	LatestDate *string `json:"latestDate"`
}

type LeagueConfigStats struct {
	Teams   int      `json:"teams"`
	Matches int      `json:"matches"`
	Players int      `json:"players"`
	Seasons []string `json:"seasons"`
}

type LeagueConfigItem struct {
	League string            `json:"league"`
	Stats  LeagueConfigStats `json:"stats"`
}

type LeagueConfigsResponse struct {
	Data []LeagueConfigItem `json:"data"`
}

type RunRateTrendItem struct {
	Season     string  `json:"season"`
	AvgRunRate float64 `json:"avgRunRate"`
	TotalRuns  int     `json:"totalRuns"`
	TotalBalls int     `json:"totalBalls"`
}

type RunRateTrendResponse struct {
	Data     []RunRateTrendItem   `json:"data"`
	Team     *string              `json:"team"`
	League   string               `json:"league"`
	Metadata RunRateTrendMetadata `json:"metadata"`
}

type RunRateTrendMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	TotalSeasons     int      `json:"totalSeasons"`
}

type TeamRunRateProgressionPoint struct {
	Over    int     `json:"over"`
	Phase   string  `json:"phase"`
	Runs    int     `json:"runs"`
	Balls   int     `json:"balls"`
	RunRate float64 `json:"runRate"`
}

type TeamRunRateProgressionResponse struct {
	Data     []TeamRunRateProgressionPoint  `json:"data"`
	Team     string                         `json:"team"`
	Season   string                         `json:"season"`
	League   string                         `json:"league"`
	Metadata TeamRunRateProgressionMetadata `json:"metadata"`
}

type TeamRunRateProgressionMetadata struct {
	TotalInnings     int      `json:"totalInnings"`
	TotalMatches     int      `json:"totalMatches"`
	TotalDeliveries  int      `json:"totalDeliveries"`
	AvailableLeagues []string `json:"availableLeagues"`
}

type PlayerCompareBatting struct {
	Runs         int     `json:"runs"`
	BallsFaced   int     `json:"ballsFaced"`
	Innings      int     `json:"innings"`
	NotOuts      int     `json:"notOuts"`
	HighestScore int     `json:"highestScore"`
	StrikeRate   float64 `json:"strikeRate"`
	Average      float64 `json:"average"`
	Fours        int     `json:"fours"`
	Sixes        int     `json:"sixes"`
	Fifties      int     `json:"fifties"`
	Hundreds     int     `json:"hundreds"`
}

type PlayerCompareBowling struct {
	Wickets      int     `json:"wickets"`
	BallsBowled  int     `json:"ballsBowled"`
	RunsConceded int     `json:"runsConceded"`
	Innings      int     `json:"innings"`
	Economy      float64 `json:"economy"`
	Average      float64 `json:"average"`
	StrikeRate   float64 `json:"strikeRate"`
	FourWickets  int     `json:"fourWickets"`
	FiveWickets  int     `json:"fiveWickets"`
}

type PlayerComparePlayer struct {
	Name    string                `json:"name"`
	Batting *PlayerCompareBatting `json:"batting,omitempty"`
	Bowling *PlayerCompareBowling `json:"bowling,omitempty"`
}

type PlayerCompareFilters struct {
	Seasons  []string `json:"seasons"`
	Team     *string  `json:"team"`
	StatType string   `json:"statType"`
}

type PlayerCompareData struct {
	Players []PlayerComparePlayer `json:"players"`
	Filters PlayerCompareFilters  `json:"filters"`
}

type PlayerCompareMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
	PlayerCount      int      `json:"playerCount"`
}

type PlayerCompareResponse struct {
	Data     PlayerCompareData     `json:"data"`
	League   string                `json:"league"`
	Metadata PlayerCompareMetadata `json:"metadata"`
}

type StatExplorerFilterOptions struct {
	Teams               []string `json:"teams"`
	Opposition          []string `json:"opposition"`
	Seasons             []string `json:"seasons"`
	Venues              []string `json:"venues"`
	Cities              []string `json:"cities"`
	TossWinners         []string `json:"tossWinners"`
	TossDecisions       []string `json:"tossDecisions"`
	Innings             []int    `json:"innings"`
	AvailableMetrics    []string `json:"availableMetrics"`
	AvailableDimensions []string `json:"availableDimensions"`
	BattingHands        []string `json:"battingHands"`
	BowlingTypes        []string `json:"bowlingTypes"`
	BowlingSubTypes     []string `json:"bowlingSubTypes"`
	PlayingRoles        []string `json:"playingRoles"`
	PlayingRoleDetails  []string `json:"playingRoleDetails"`
	BattingPositions    []int    `json:"battingPositions"`
}

type StatExplorerOptionsMetadata struct {
	ReportType string `json:"reportType"`
}

type StatExplorerOptionsResponse struct {
	Options  StatExplorerFilterOptions   `json:"options"`
	League   string                      `json:"league"`
	Metadata StatExplorerOptionsMetadata `json:"metadata"`
}

type StatExplorerSort struct {
	Key       string `json:"key"`
	Direction string `json:"direction"`
}

type StatExplorerRunFilters struct {
	Teams                  []string `json:"teams,omitempty"`
	Opposition             []string `json:"opposition,omitempty"`
	Seasons                []string `json:"seasons,omitempty"`
	DateFrom               *string  `json:"dateFrom,omitempty"`
	DateTo                 *string  `json:"dateTo,omitempty"`
	Venues                 []string `json:"venues,omitempty"`
	Cities                 []string `json:"cities,omitempty"`
	TossWinners            []string `json:"tossWinners,omitempty"`
	TossDecisions          []string `json:"tossDecisions,omitempty"`
	Innings                []int    `json:"innings,omitempty"`
	OverFrom               *int     `json:"overFrom,omitempty"`
	OverTo                 *int     `json:"overTo,omitempty"`
	Phase                  *string  `json:"phase,omitempty"`
	ResultFilter           *string  `json:"resultFilter,omitempty"`
	MinRuns                *int     `json:"minRuns,omitempty"`
	MaxRuns                *int     `json:"maxRuns,omitempty"`
	MinBalls               *int     `json:"minBalls,omitempty"`
	MaxBalls               *int     `json:"maxBalls,omitempty"`
	MinWickets             *int     `json:"minWickets,omitempty"`
	MaxWickets             *int     `json:"maxWickets,omitempty"`
	BattingHand            *string  `json:"battingHand,omitempty"`
	BowlingType            *string  `json:"bowlingType,omitempty"`
	BowlingSubType         []string `json:"bowlingSubType,omitempty"`
	OpponentBattingHand    *string  `json:"opponentBattingHand,omitempty"`
	OpponentBowlingType    *string  `json:"opponentBowlingType,omitempty"`
	OpponentBowlingSubType []string `json:"opponentBowlingSubType,omitempty"`
	PlayingRole            *string  `json:"playingRole,omitempty"`
	PlayingRoleDetail      *string  `json:"playingRoleDetail,omitempty"`
	BattingPositions       []int    `json:"battingPositions,omitempty"`
}

type StatExplorerRunPagination struct {
	Page     int `json:"page"`
	PageSize int `json:"pageSize"`
}

type StatExplorerRunRequest struct {
	ReportType string                    `json:"reportType"`
	Dimensions []string                  `json:"dimensions"`
	Metrics    []string                  `json:"metrics"`
	Filters    StatExplorerRunFilters    `json:"filters"`
	Sort       *StatExplorerSort         `json:"sort,omitempty"`
	Pagination StatExplorerRunPagination `json:"pagination"`
}

type StatExplorerColumn struct {
	Key       string `json:"key"`
	Label     string `json:"label"`
	IsNumeric bool   `json:"isNumeric"`
}

type StatExplorerRunResult struct {
	Data      []map[string]interface{} `json:"data"`
	Columns   []StatExplorerColumn     `json:"columns"`
	TotalRows int                      `json:"totalRows"`
}

type StatExplorerRunMetadata struct {
	AvailableLeagues []string               `json:"availableLeagues"`
	ReportType       string                 `json:"reportType"`
	Filters          StatExplorerRunFilters `json:"filters"`
	Dimensions       []string               `json:"dimensions"`
	Metrics          []string               `json:"metrics"`
}

type StatExplorerRunResponse struct {
	Data       []map[string]interface{} `json:"data"`
	Columns    []StatExplorerColumn     `json:"columns"`
	TotalRows  int                      `json:"totalRows"`
	Page       int                      `json:"page"`
	PageSize   int                      `json:"pageSize"`
	TotalPages int                      `json:"totalPages"`
	League     string                   `json:"league"`
	Metadata   StatExplorerRunMetadata  `json:"metadata"`
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
	Images         []string   `json:"images,omitempty"`
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
	ID            int    `json:"id"`
	League        string `json:"league"`
	Season        string `json:"season"`
	StartDate     string `json:"startDate"`
	Venue         string `json:"venue"`
	Team1         string `json:"team1"`
	Team2         string `json:"team2"`
	Innings1Score string `json:"innings1Score"`
	Innings2Score string `json:"innings2Score"`
	Result        string `json:"result"`
}

type MatchesResponse struct {
	Matches    []MatchCard     `json:"matches"`
	League     string          `json:"league"`
	Pagination Pagination      `json:"pagination"`
	Seasons    []string        `json:"seasons"`
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
	Data     []TeamAverageItem    `json:"data"`
	League   string               `json:"league"`
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
	Opponent    string  `json:"opponent"`
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

type MatchupRoundResponse struct {
	Batter          string             `json:"batter"`
	Prompt          string             `json:"prompt"`
	QuestionType    string             `json:"questionType"`
	CorrectOpponent string             `json:"correctOpponent"`
	Options         []MultiMatchupItem `json:"options"`
	League          string             `json:"league"`
	Metadata        MatchupRoundMetadata `json:"metadata"`
}

type MatchupRoundMetadata struct {
	AvailableLeagues []string `json:"availableLeagues"`
}

type PlayerProgressionPoint struct {
	Over       int      `json:"over"`
	Phase      string   `json:"phase"`
	Runs       int      `json:"runs"`
	Balls      int      `json:"balls"`
	Dismissals int      `json:"dismissals"`
	StrikeRate float64  `json:"strikeRate"`
	Average    *float64 `json:"average"`
}

type PlayerProgressionResponse struct {
	Data     []PlayerProgressionPoint  `json:"data"`
	Player   string                    `json:"player"`
	League   string                    `json:"league"`
	Innings  *string                   `json:"innings"`
	Metadata PlayerProgressionMetadata `json:"metadata"`
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
	Data       interface{}           `json:"data"`
	League     string                `json:"league"`
	Player     string                `json:"player"`
	PlayerType string                `json:"playerType"`
	Overs      []int                 `json:"overs"`
	Metadata   AdvancedStatsMetadata `json:"metadata"`
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
	InningsNumber int                  `json:"inningsNumber"`
	BattingTeam   string               `json:"battingTeam"`
	Wickets       []FallOfWicketsEntry `json:"wickets"`
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
