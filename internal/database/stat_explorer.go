package database

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

var statExplorerDimensionLabels = map[string]string{
	"season":                 "Season",
	"player":                 "Player",
	"team":                   "Team",
	"opposition":             "Opposition",
	"venue":                  "Venue",
	"city":                   "City",
	"tosswinner":             "Toss Winner",
	"tossdecision":           "Toss Decision",
	"result":                 "Result",
	"date":                   "Date",
	"innings":                "Innings",
	"battinghand":            "Player Batting Hand",
	"bowlingtype":            "Player Bowling Type",
	"bowlingsubtype":         "Player Bowling Sub-Type",
	"opponentbattinghand":    "Opponent Batting Hand",
	"opponentbowlingtype":    "Opponent Bowling Type",
	"opponentbowlingsubtype": "Opponent Bowling Sub-Type",
	"playingrole":            "Playing Role",
}

var statExplorerMetricLabels = map[string]string{
	"runs":              "Runs",
	"ballsfaced":        "Balls Faced",
	"innings":           "Innings",
	"notouts":           "Not Outs",
	"highestscore":      "Highest Score",
	"fours":             "Fours",
	"sixes":             "Sixes",
	"fifties":           "50s",
	"hundreds":          "100s",
	"strikerate":        "Strike Rate",
	"average":           "Average",
	"dismissals":        "Dismissals",
	"dotballs":          "Dot Balls",
	"wickets":           "Wickets",
	"ballsbowled":       "Balls Bowled",
	"runsconceded":      "Runs Conceded",
	"economyrate":       "Economy Rate",
	"bowlingaverage":    "Bowling Average",
	"bowlingstrikerate": "Bowling SR",
	"fourwickets":       "4 Wickets",
	"fivewickets":       "5 Wickets",
	"matchesplayed":     "Matches",
	"wins":              "Wins",
	"losses":            "Losses",
	"winpct":            "Win %",
	"matches":           "Matches",
	"winsbattingfirst":  "Batting 1st Wins",
	"winsbattingsecond": "Batting 2nd Wins",
}

type statExplorerSQLBuilder struct {
	args []interface{}
}

type statExplorerBuiltQuery struct {
	sql        string
	countSQL   string
	args       []interface{}
	metricKeys map[string]bool
}

func (b *statExplorerSQLBuilder) addArg(value interface{}) string {
	b.args = append(b.args, value)
	return fmt.Sprintf("$%d", len(b.args))
}

func (b *statExplorerSQLBuilder) stringInClause(values []string) string {
	placeholders := make([]string, 0, len(values))
	for _, value := range values {
		placeholders = append(placeholders, b.addArg(value))
	}
	return fmt.Sprintf("(%s)", strings.Join(placeholders, ", "))
}

func (b *statExplorerSQLBuilder) intInClause(values []int) string {
	placeholders := make([]string, 0, len(values))
	for _, value := range values {
		placeholders = append(placeholders, b.addArg(value))
	}
	return fmt.Sprintf("(%s)", strings.Join(placeholders, ", "))
}

func (s *service) RunStatExplorer(
	ctx context.Context,
	league string,
	request models.StatExplorerRunRequest,
) (models.StatExplorerRunResult, error) {
	built, err := buildStatExplorerQueries(request, league)
	if err != nil {
		return models.StatExplorerRunResult{}, err
	}

	data, columns, err := s.queryStatExplorerRows(ctx, built.sql, built.args, built.metricKeys)
	if err != nil {
		return models.StatExplorerRunResult{}, err
	}

	var totalRows int
	if err := s.db.QueryRowContext(ctx, built.countSQL, built.args...).Scan(&totalRows); err != nil {
		return models.StatExplorerRunResult{}, err
	}

	return models.StatExplorerRunResult{
		Data:      data,
		Columns:   columns,
		TotalRows: totalRows,
	}, nil
}

func (s *service) queryStatExplorerRows(
	ctx context.Context,
	query string,
	args []interface{},
	metricKeys map[string]bool,
) ([]map[string]interface{}, []models.StatExplorerColumn, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	columnNames, err := rows.Columns()
	if err != nil {
		return nil, nil, err
	}

	data := []map[string]interface{}{}
	columns := []models.StatExplorerColumn{}

	for rows.Next() {
		values := make([]interface{}, len(columnNames))
		valuePtrs := make([]interface{}, len(columnNames))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, nil, err
		}

		row := make(map[string]interface{}, len(columnNames))
		for i, name := range columnNames {
			lowerName := strings.ToLower(name)
			row[name] = normalizeStatExplorerValue(values[i], metricKeys[lowerName])
		}
		data = append(data, row)

		if len(columns) == 0 {
			for _, name := range columnNames {
				lowerName := strings.ToLower(name)
				label := name
				if value, ok := statExplorerDimensionLabels[lowerName]; ok {
					label = value
				} else if value, ok := statExplorerMetricLabels[lowerName]; ok {
					label = value
				}
				columns = append(columns, models.StatExplorerColumn{
					Key:       name,
					Label:     label,
					IsNumeric: metricKeys[lowerName],
				})
			}
		}
	}

	if err := rows.Err(); err != nil {
		return nil, nil, err
	}

	if len(data) == 0 {
		columns = []models.StatExplorerColumn{}
	}

	return data, columns, nil
}

func buildStatExplorerQueries(request models.StatExplorerRunRequest, league string) (statExplorerBuiltQuery, error) {
	builder := &statExplorerSQLBuilder{}
	switch request.ReportType {
	case "team":
		return buildTeamReportQueries(builder, request, league)
	case "match":
		return buildMatchReportQueries(builder, request, league)
	default:
		return buildBattingBowlingQueries(builder, request, league)
	}
}

func buildBattingBowlingQueries(
	builder *statExplorerSQLBuilder,
	request models.StatExplorerRunRequest,
	league string,
) (statExplorerBuiltQuery, error) {
	isBowling := request.ReportType == "bowling"
	whereClause := buildStatExplorerWhereClause(builder, request.ReportType, request.Filters, league)

	primaryStyleDims := map[string]bool{"battingHand": true, "bowlingType": true, "bowlingSubType": true, "playingRole": true}
	opponentStyleDims := map[string]bool{"opponentBattingHand": true, "opponentBowlingType": true, "opponentBowlingSubType": true}
	hasPrimaryStyleDim := false
	hasOpponentStyleDim := false
	for _, dimension := range request.Dimensions {
		if primaryStyleDims[dimension] {
			hasPrimaryStyleDim = true
		}
		if opponentStyleDims[dimension] {
			hasOpponentStyleDim = true
		}
	}
	hasPrimaryStyleFilter := request.Filters.BattingHand != nil || request.Filters.BowlingType != nil || len(request.Filters.BowlingSubType) > 0 || request.Filters.PlayingRole != nil || request.Filters.PlayingRoleDetail != nil
	hasOpponentStyleFilter := request.Filters.OpponentBattingHand != nil || request.Filters.OpponentBowlingType != nil || len(request.Filters.OpponentBowlingSubType) > 0
	needsPrimaryPlayerStyle := hasPrimaryStyleDim || hasPrimaryStyleFilter
	needsOpponentPlayerStyle := hasOpponentStyleDim || hasOpponentStyleFilter
	needsPlayerStyle := needsPrimaryPlayerStyle || needsOpponentPlayerStyle

	playerStyleLookupCTE := ""
	if needsPlayerStyle {
		playerStyleLookupCTE = `player_style_lookup AS (
			SELECT DISTINCT ON (pr.person_name)
				pr.person_name,
				ps.batting_hand,
				ps.bowling_hand,
				ps.bowling_type,
				ps.bowling_sub_type,
				ps.playing_role,
				ps.playing_role_detail
			FROM wpl_person_registry pr
			JOIN player_style ps ON ps.identifier = pr.registry_id
			ORDER BY pr.person_name
		)`
	}

	playerStyleConditions := []string{}
	if request.Filters.BattingHand != nil {
		playerStyleConditions = append(playerStyleConditions, fmt.Sprintf("primary_psl.batting_hand = %s", builder.addArg(*request.Filters.BattingHand)))
	}
	if request.Filters.BowlingType != nil {
		playerStyleConditions = append(playerStyleConditions, fmt.Sprintf("primary_psl.bowling_type = %s", builder.addArg(*request.Filters.BowlingType)))
	}
	if len(request.Filters.BowlingSubType) > 0 {
		playerStyleConditions = append(playerStyleConditions, fmt.Sprintf("primary_psl.bowling_sub_type IN %s", builder.stringInClause(request.Filters.BowlingSubType)))
	}
	if request.Filters.PlayingRole != nil {
		playerStyleConditions = append(playerStyleConditions, fmt.Sprintf("primary_psl.playing_role = %s", builder.addArg(*request.Filters.PlayingRole)))
	}
	if request.Filters.PlayingRoleDetail != nil {
		playerStyleConditions = append(playerStyleConditions, fmt.Sprintf("primary_psl.playing_role_detail = %s", builder.addArg(*request.Filters.PlayingRoleDetail)))
	}
	if request.Filters.OpponentBattingHand != nil {
		playerStyleConditions = append(playerStyleConditions, fmt.Sprintf("opponent_psl.batting_hand = %s", builder.addArg(*request.Filters.OpponentBattingHand)))
	}
	if request.Filters.OpponentBowlingType != nil {
		playerStyleConditions = append(playerStyleConditions, fmt.Sprintf("opponent_psl.bowling_type = %s", builder.addArg(*request.Filters.OpponentBowlingType)))
	}
	if len(request.Filters.OpponentBowlingSubType) > 0 {
		playerStyleConditions = append(playerStyleConditions, fmt.Sprintf("opponent_psl.bowling_sub_type IN %s", builder.stringInClause(request.Filters.OpponentBowlingSubType)))
	}

	playerStyleFilterSQL := ""
	if len(playerStyleConditions) > 0 {
		playerStyleFilterSQL = "AND " + strings.Join(playerStyleConditions, " AND ")
	}

	primaryJoinSQL := ""
	if needsPrimaryPlayerStyle {
		if isBowling {
			primaryJoinSQL = "LEFT JOIN player_style_lookup primary_psl ON primary_psl.person_name = d.bowler"
		} else {
			primaryJoinSQL = "LEFT JOIN player_style_lookup primary_psl ON primary_psl.person_name = d.striker"
		}
	}
	opponentJoinSQL := ""
	if needsOpponentPlayerStyle {
		if isBowling {
			opponentJoinSQL = "LEFT JOIN player_style_lookup opponent_psl ON opponent_psl.person_name = d.striker"
		} else {
			opponentJoinSQL = "LEFT JOIN player_style_lookup opponent_psl ON opponent_psl.person_name = d.bowler"
		}
	}

	primarySelectSQL := ""
	if needsPrimaryPlayerStyle {
		primarySelectSQL = `,
			MAX(primary_psl.batting_hand) AS batting_hand,
			MAX(primary_psl.bowling_type) AS bowling_type,
			MAX(primary_psl.bowling_sub_type) AS bowling_sub_type,
			MAX(primary_psl.playing_role) AS playing_role`
	}

	opponentDimensionSelects := []string{}
	opponentDimensionGroups := []string{}
	for _, dimension := range request.Dimensions {
		switch dimension {
		case "opponentBattingHand":
			opponentDimensionSelects = append(opponentDimensionSelects, "opponent_psl.batting_hand AS opponent_batting_hand")
			opponentDimensionGroups = append(opponentDimensionGroups, "opponent_psl.batting_hand")
		case "opponentBowlingType":
			opponentDimensionSelects = append(opponentDimensionSelects, "opponent_psl.bowling_type AS opponent_bowling_type")
			opponentDimensionGroups = append(opponentDimensionGroups, "opponent_psl.bowling_type")
		case "opponentBowlingSubType":
			opponentDimensionSelects = append(opponentDimensionSelects, "opponent_psl.bowling_sub_type AS opponent_bowling_sub_type")
			opponentDimensionGroups = append(opponentDimensionGroups, "opponent_psl.bowling_sub_type")
		}
	}
	opponentSelectSQL := ""
	if len(opponentDimensionSelects) > 0 {
		opponentSelectSQL = ",\n\t\t\t" + strings.Join(opponentDimensionSelects, ",\n\t\t\t")
	}
	opponentGroupBySQL := ""
	if len(opponentDimensionGroups) > 0 {
		opponentGroupBySQL = ", " + strings.Join(opponentDimensionGroups, ", ")
	}

	groupByParts := []string{}
	selectDimParts := []string{}
	sortColumns := map[string]string{}
	for _, dimension := range request.Dimensions {
		switch dimension {
		case "season":
			groupByParts = append(groupByParts, "stats.season")
			selectDimParts = append(selectDimParts, `stats.season AS season`)
		case "player":
			groupByParts = append(groupByParts, "stats.player")
			selectDimParts = append(selectDimParts, `stats.player AS player`)
		case "team":
			groupByParts = append(groupByParts, "stats.team")
			selectDimParts = append(selectDimParts, `stats.team AS team`)
		case "opposition":
			groupByParts = append(groupByParts, "stats.opposition")
			selectDimParts = append(selectDimParts, `stats.opposition AS opposition`)
		case "venue":
			groupByParts = append(groupByParts, "stats.venue")
			selectDimParts = append(selectDimParts, `stats.venue AS venue`)
		case "city":
			groupByParts = append(groupByParts, "stats.city")
			selectDimParts = append(selectDimParts, `stats.city AS city`)
		case "tossWinner":
			groupByParts = append(groupByParts, "stats.toss_winner")
			selectDimParts = append(selectDimParts, `stats.toss_winner AS tossWinner`)
		case "tossDecision":
			groupByParts = append(groupByParts, "stats.toss_decision")
			selectDimParts = append(selectDimParts, `stats.toss_decision AS tossDecision`)
		case "result":
			groupByParts = append(groupByParts, "stats.match_winner")
			selectDimParts = append(selectDimParts, `stats.match_winner AS result`)
		case "date":
			groupByParts = append(groupByParts, "stats.start_date::date")
			selectDimParts = append(selectDimParts, `stats.start_date::date AS date_col`)
		case "innings":
			groupByParts = append(groupByParts, "stats.innings")
			selectDimParts = append(selectDimParts, `stats.innings AS innings`)
		case "battingHand":
			groupByParts = append(groupByParts, "stats.batting_hand")
			selectDimParts = append(selectDimParts, `stats.batting_hand AS "battingHand"`)
		case "bowlingType":
			groupByParts = append(groupByParts, "stats.bowling_type")
			selectDimParts = append(selectDimParts, `stats.bowling_type AS "bowlingType"`)
		case "bowlingSubType":
			groupByParts = append(groupByParts, "stats.bowling_sub_type")
			selectDimParts = append(selectDimParts, `stats.bowling_sub_type AS "bowlingSubType"`)
		case "opponentBattingHand":
			groupByParts = append(groupByParts, "stats.opponent_batting_hand")
			selectDimParts = append(selectDimParts, `stats.opponent_batting_hand AS "opponentBattingHand"`)
		case "opponentBowlingType":
			groupByParts = append(groupByParts, "stats.opponent_bowling_type")
			selectDimParts = append(selectDimParts, `stats.opponent_bowling_type AS "opponentBowlingType"`)
		case "opponentBowlingSubType":
			groupByParts = append(groupByParts, "stats.opponent_bowling_sub_type")
			selectDimParts = append(selectDimParts, `stats.opponent_bowling_sub_type AS "opponentBowlingSubType"`)
		case "playingRole":
			groupByParts = append(groupByParts, "stats.playing_role")
			selectDimParts = append(selectDimParts, `stats.playing_role AS "playingRole"`)
		}
		sortColumns[strings.ToLower(dimension)] = statExplorerSortColumnForDimension(dimension)
	}

	selectMetricParts := make([]string, 0, len(request.Metrics))
	metricKeys := map[string]bool{}
	for _, metric := range request.Metrics {
		selectMetricParts = append(selectMetricParts, fmt.Sprintf(`%s AS %s`, statExplorerMetricSQL(metric, isBowling), strings.ToLower(metric)))
		metricKeys[strings.ToLower(metric)] = true
		sortColumns[strings.ToLower(metric)] = strings.ToLower(metric)
	}

	allSelectParts := append(selectDimParts, selectMetricParts...)
	groupByClause := ""
	if len(groupByParts) > 0 {
		groupByClause = "GROUP BY " + strings.Join(groupByParts, ", ")
	}
	defaultOrderBy := fmt.Sprintf("ORDER BY %s DESC", strings.ToLower(request.Metrics[0]))
	orderBy := buildStatExplorerOrderByClause(request.Sort, sortColumns, defaultOrderBy)

	statsCTE := ""
	if isBowling {
		statsCTE = fmt.Sprintf(`stats AS (
			SELECT
				d.match_id,
				d.innings,
				d.bowler AS player,
				MAX(m.season) AS season,
				MAX(m.start_date) AS start_date,
				MAX(m.venue) AS venue,
				MAX(mi.city) AS city,
				MAX(mi.toss_winner) AS toss_winner,
				MAX(mi.toss_decision) AS toss_decision,
				MAX(mi.winner) AS match_winner,
				MAX(d.bowling_team) AS team,
				MAX(%s) AS opposition,
				COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket')) AS wickets,
				COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0) AS balls_bowled,
				SUM(d.runs_off_bat + d.wides + d.noballs) AS runs_conceded,
				COUNT(*) FILTER (WHERE d.runs_off_bat = 0 AND d.wides = 0 AND d.noballs = 0) AS dot_balls%s%s
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			LEFT JOIN wpl_match_info mi ON m.match_id = mi.match_id
			%s
			%s
			WHERE %s
				%s
			GROUP BY d.match_id, d.innings, d.bowler%s
		)`, standardizedBattingTeamSQL, primarySelectSQL, opponentSelectSQL, primaryJoinSQL, opponentJoinSQL, whereClause, playerStyleFilterSQL, opponentGroupBySQL)
	} else {
		statsCTE = fmt.Sprintf(`stats AS (
			SELECT
				d.match_id,
				d.innings,
				d.striker AS player,
				MAX(m.season) AS season,
				MAX(m.start_date) AS start_date,
				MAX(m.venue) AS venue,
				MAX(mi.city) AS city,
				MAX(mi.toss_winner) AS toss_winner,
				MAX(mi.toss_decision) AS toss_decision,
				MAX(mi.winner) AS match_winner,
				MAX(%s) AS team,
				MAX(d.bowling_team) AS opposition,
				SUM(d.runs_off_bat) AS runs,
				COUNT(*) FILTER (WHERE d.wides = 0) AS balls_faced,
				COUNT(*) FILTER (WHERE d.runs_off_bat = 4) AS fours,
				COUNT(*) FILTER (WHERE d.runs_off_bat = 6) AS sixes,
				COUNT(*) FILTER (WHERE d.runs_off_bat = 0 AND d.wides = 0 AND d.noballs = 0) AS dot_balls,
				MAX(CASE WHEN d.player_dismissed = d.striker AND d.wicket_type IN ('caught', 'bowled', 'lbw', 'stumped', 'caught and bowled', 'hit wicket', 'run out', 'retired out', 'obstructing the field', 'hit the ball twice', 'handled the ball', 'timed out') THEN 1 ELSE 0 END) AS is_dismissed%s%s
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			LEFT JOIN wpl_match_info mi ON m.match_id = mi.match_id
			%s
			%s
			WHERE %s
				%s
			GROUP BY d.match_id, d.innings, d.striker%s
		)`, standardizedBattingTeamSQL, primarySelectSQL, opponentSelectSQL, primaryJoinSQL, opponentJoinSQL, whereClause, playerStyleFilterSQL, opponentGroupBySQL)
	}

	ctes := []string{}
	if playerStyleLookupCTE != "" {
		ctes = append(ctes, playerStyleLookupCTE)
	}
	ctes = append(ctes, statsCTE)
	withClause := "WITH " + strings.Join(ctes, ",\n")

	offset := (request.Pagination.Page - 1) * request.Pagination.PageSize
	sql := fmt.Sprintf(`
		%s
		SELECT %s
		FROM stats
		%s
		%s
		LIMIT %d OFFSET %d
	`, withClause, strings.Join(allSelectParts, ",\n"), groupByClause, orderBy, request.Pagination.PageSize, offset)

	countSQL := fmt.Sprintf(`
		%s
		SELECT COUNT(*)::int AS total
		FROM (
			SELECT 1
			FROM stats
			%s
		) grouped
	`, withClause, groupByClause)

	return statExplorerBuiltQuery{sql: sql, countSQL: countSQL, args: builder.args, metricKeys: metricKeys}, nil
}

func buildTeamReportQueries(
	builder *statExplorerSQLBuilder,
	request models.StatExplorerRunRequest,
	league string,
) (statExplorerBuiltQuery, error) {
	conditions := []string{fmt.Sprintf("m.league = %s", builder.addArg(league))}
	if len(request.Filters.Seasons) > 0 {
		conditions = append(conditions, fmt.Sprintf("m.season IN %s", builder.stringInClause(request.Filters.Seasons)))
	}
	if request.Filters.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("m.start_date >= %s", builder.addArg(*request.Filters.DateFrom)))
	}
	if request.Filters.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("m.start_date <= %s", builder.addArg(*request.Filters.DateTo)))
	}
	if len(request.Filters.Venues) > 0 {
		conditions = append(conditions, fmt.Sprintf("m.venue IN %s", builder.stringInClause(request.Filters.Venues)))
	}
	if len(request.Filters.Cities) > 0 {
		conditions = append(conditions, fmt.Sprintf("mi.city IN %s", builder.stringInClause(request.Filters.Cities)))
	}
	whereClause := strings.Join(conditions, " AND ")

	teamDim := containsString(request.Dimensions, "team")
	seasonDim := containsString(request.Dimensions, "season")
	venueDim := containsString(request.Dimensions, "venue")
	cityDim := containsString(request.Dimensions, "city")

	groupByCols := []string{}
	selectCols := []string{}
	sortColumns := map[string]string{}
	if teamDim {
		groupByCols = append(groupByCols, "t.std_team")
		selectCols = append(selectCols, `t.std_team AS "team"`)
		sortColumns["team"] = `"team"`
	}
	if seasonDim {
		groupByCols = append(groupByCols, "m.season")
		selectCols = append(selectCols, `m.season AS "season"`)
		sortColumns["season"] = `"season"`
	}
	if venueDim {
		groupByCols = append(groupByCols, "m.venue")
		selectCols = append(selectCols, `m.venue AS "venue"`)
		sortColumns["venue"] = `"venue"`
	}
	if cityDim {
		groupByCols = append(groupByCols, "mi.city")
		selectCols = append(selectCols, `mi.city AS "city"`)
		sortColumns["city"] = `"city"`
	}

	metricKeys := map[string]bool{}
	metricExpressions := map[string]string{
		"matchesPlayed":     `COUNT(DISTINCT t.match_id)`,
		"wins":              `COUNT(DISTINCT t.match_id) FILTER (WHERE t.std_team = w.winner)`,
		"losses":            `COUNT(DISTINCT t.match_id) FILTER (WHERE t.std_team != w.winner AND w.winner IS NOT NULL)`,
		"winPct":            `CASE WHEN COUNT(DISTINCT t.match_id) FILTER (WHERE w.winner IS NOT NULL) > 0 THEN ROUND((COUNT(DISTINCT t.match_id) FILTER (WHERE t.std_team = w.winner)::numeric / COUNT(DISTINCT t.match_id) FILTER (WHERE w.winner IS NOT NULL)) * 100, 2) ELSE 0 END`,
		"winsBattingFirst":  `COUNT(DISTINCT t.match_id) FILTER (WHERE t.std_team = w.winner AND w.win_type = 'batting_first')`,
		"winsBattingSecond": `COUNT(DISTINCT t.match_id) FILTER (WHERE t.std_team = w.winner AND w.win_type = 'batting_second')`,
	}
	for _, metric := range request.Metrics {
		selectCols = append(selectCols, fmt.Sprintf(`%s AS %s`, metricExpressions[metric], quoteIdentifier(metric)))
		sortColumns[strings.ToLower(metric)] = quoteIdentifier(metric)
		metricKeys[strings.ToLower(metric)] = true
	}

	groupByClause := ""
	if len(groupByCols) > 0 {
		groupByClause = "GROUP BY " + strings.Join(groupByCols, ", ")
	}
	defaultOrderBy := fmt.Sprintf("ORDER BY %s DESC", quoteIdentifier(request.Metrics[0]))
	orderBy := buildStatExplorerOrderByClause(request.Sort, sortColumns, defaultOrderBy)
	offset := (request.Pagination.Page - 1) * request.Pagination.PageSize

	sql := fmt.Sprintf(`
		WITH delivery_std AS (
			SELECT d.*, %s AS std_team
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			LEFT JOIN wpl_match_info mi ON m.match_id = mi.match_id
			WHERE %s AND d.innings <= 2
		),
		runs_per_innings AS (
			SELECT match_id, innings, std_team, SUM(runs_off_bat + extras) AS runs
			FROM delivery_std
			GROUP BY match_id, innings, std_team
		),
		match_totals AS (
			SELECT r1.match_id, r1.std_team AS team1, r1.runs AS runs1, r2.std_team AS team2, r2.runs AS runs2
			FROM runs_per_innings r1
			JOIN runs_per_innings r2 ON r1.match_id = r2.match_id AND r1.innings = 1 AND r2.innings = 2
		),
		winners AS (
			SELECT match_id,
				CASE WHEN runs1 > runs2 THEN team1 ELSE team2 END AS winner,
				CASE WHEN runs1 > runs2 THEN team2 ELSE team1 END AS loser,
				CASE WHEN runs1 > runs2 THEN 'batting_first' ELSE 'batting_second' END AS win_type
			FROM match_totals
		),
		teams AS (
			SELECT match_id, team1 AS std_team FROM match_totals
			UNION ALL
			SELECT match_id, team2 AS std_team FROM match_totals
		)
		SELECT %s
		FROM teams t
		JOIN wpl_match m ON t.match_id = m.match_id
		LEFT JOIN wpl_match_info mi ON t.match_id = mi.match_id
		LEFT JOIN winners w ON t.match_id = w.match_id
		%s
		%s
		LIMIT %d OFFSET %d
	`, standardizedBattingTeamSQL, whereClause, strings.Join(selectCols, ",\n"), groupByClause, orderBy, request.Pagination.PageSize, offset)

	groupedSubquery := "SELECT 1 FROM teams t LEFT JOIN winners w ON t.match_id = w.match_id"
	if len(groupByCols) > 0 {
		groupedSubquery = fmt.Sprintf(`SELECT 1
			FROM teams t
			JOIN wpl_match m ON t.match_id = m.match_id
			LEFT JOIN wpl_match_info mi ON t.match_id = mi.match_id
			LEFT JOIN winners w ON t.match_id = w.match_id
			%s`, groupByClause)
	}

	countSQL := fmt.Sprintf(`
		WITH delivery_std AS (
			SELECT d.*
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			LEFT JOIN wpl_match_info mi ON m.match_id = mi.match_id
			WHERE %s AND d.innings <= 2
		),
		runs_per_innings AS (
			SELECT match_id, innings, %s AS std_team, SUM(runs_off_bat + extras) AS runs
			FROM delivery_std d
			GROUP BY match_id, innings, %s
		),
		match_totals AS (
			SELECT r1.match_id, r1.std_team AS team1, r1.runs AS runs1, r2.std_team AS team2, r2.runs AS runs2
			FROM runs_per_innings r1
			JOIN runs_per_innings r2 ON r1.match_id = r2.match_id AND r1.innings = 1 AND r2.innings = 2
		),
		winners AS (
			SELECT match_id,
				CASE WHEN runs1 > runs2 THEN team1 ELSE team2 END AS winner,
				CASE WHEN runs1 > runs2 THEN team2 ELSE team1 END AS loser,
				CASE WHEN runs1 > runs2 THEN 'batting_first' ELSE 'batting_second' END AS win_type
			FROM match_totals
		),
		teams AS (
			SELECT match_id, team1 AS std_team FROM match_totals
			UNION ALL
			SELECT match_id, team2 AS std_team FROM match_totals
		)
		SELECT COUNT(*)::int AS total
		FROM (%s) grouped
	`, whereClause, standardizedBattingTeamSQL, standardizedBattingTeamSQL, groupedSubquery)

	return statExplorerBuiltQuery{sql: sql, countSQL: countSQL, args: builder.args, metricKeys: metricKeys}, nil
}

func buildMatchReportQueries(
	builder *statExplorerSQLBuilder,
	request models.StatExplorerRunRequest,
	league string,
) (statExplorerBuiltQuery, error) {
	conditions := []string{fmt.Sprintf("m.league = %s", builder.addArg(league))}
	if len(request.Filters.Teams) > 0 {
		inClause := builder.stringInClause(request.Filters.Teams)
		conditions = append(conditions, fmt.Sprintf("(d.batting_team IN %s OR d.bowling_team IN %s)", inClause, inClause))
	}
	if len(request.Filters.Opposition) > 0 {
		inClause := builder.stringInClause(request.Filters.Opposition)
		conditions = append(conditions, fmt.Sprintf("(d.batting_team IN %s OR d.bowling_team IN %s)", inClause, inClause))
	}
	if len(request.Filters.Seasons) > 0 {
		conditions = append(conditions, fmt.Sprintf("m.season IN %s", builder.stringInClause(request.Filters.Seasons)))
	}
	if request.Filters.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("m.start_date >= %s", builder.addArg(*request.Filters.DateFrom)))
	}
	if request.Filters.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("m.start_date <= %s", builder.addArg(*request.Filters.DateTo)))
	}
	if len(request.Filters.Venues) > 0 {
		conditions = append(conditions, fmt.Sprintf("m.venue IN %s", builder.stringInClause(request.Filters.Venues)))
	}
	if len(request.Filters.Cities) > 0 {
		conditions = append(conditions, fmt.Sprintf("mi.city IN %s", builder.stringInClause(request.Filters.Cities)))
	}
	if len(request.Filters.TossWinners) > 0 {
		conditions = append(conditions, fmt.Sprintf("mi.toss_winner IN %s", builder.stringInClause(request.Filters.TossWinners)))
	}
	if len(request.Filters.TossDecisions) > 0 {
		conditions = append(conditions, fmt.Sprintf("mi.toss_decision IN %s", builder.stringInClause(request.Filters.TossDecisions)))
	}
	if request.Filters.ResultFilter != nil {
		switch *request.Filters.ResultFilter {
		case "won":
			conditions = append(conditions, fmt.Sprintf("mi.winner IS NOT NULL AND mi.winner = %s", standardizedBattingTeamSQL))
		case "lost":
			conditions = append(conditions, fmt.Sprintf("mi.winner IS NOT NULL AND mi.winner != %s", standardizedBattingTeamSQL))
		case "noresult":
			conditions = append(conditions, "mi.winner IS NULL")
		}
	}
	whereClause := strings.Join(conditions, " AND ")

	groupByCols := []string{"m.match_id"}
	for _, dimension := range request.Dimensions {
		switch dimension {
		case "team":
			groupByCols = append(groupByCols, "d.batting_team")
		case "season":
			groupByCols = append(groupByCols, "m.season")
		case "venue":
			groupByCols = append(groupByCols, "m.venue")
		case "city":
			groupByCols = append(groupByCols, "mi.city")
		case "tossWinner":
			groupByCols = append(groupByCols, "mi.toss_winner")
		case "tossDecision":
			groupByCols = append(groupByCols, "mi.toss_decision")
		case "result":
			groupByCols = append(groupByCols, "mi.winner")
		case "innings":
			groupByCols = append(groupByCols, "d.innings")
		}
	}

	selectCols := []string{
		`m.match_id AS "match_id"`,
		`m.season AS "season"`,
		`m.start_date::date AS "date"`,
		`m.venue AS "venue"`,
		`mi.city AS "city"`,
		`mi.toss_winner AS "tossWinner"`,
		`mi.toss_decision AS "tossDecision"`,
		`mi.winner AS "result"`,
	}
	sortColumns := map[string]string{
		"match_id":     `"match_id"`,
		"season":       `"season"`,
		"date":         `"date"`,
		"venue":        `"venue"`,
		"city":         `"city"`,
		"tosswinner":   `"tossWinner"`,
		"tossdecision": `"tossDecision"`,
		"result":       `"result"`,
	}
	metricKeys := map[string]bool{}
	for _, metric := range request.Metrics {
		metricKeys[strings.ToLower(metric)] = true
		sortColumns[strings.ToLower(metric)] = quoteIdentifier(metric)
		switch metric {
		case "matches":
			selectCols = append(selectCols, `COUNT(DISTINCT m.match_id) AS "matches"`)
		case "runs":
			selectCols = append(selectCols, `SUM(d.runs_off_bat) AS "runs"`)
		case "wickets":
			selectCols = append(selectCols, `COUNT(*) FILTER (WHERE d.player_dismissed IS NOT NULL) AS "wickets"`)
		case "ballsFaced":
			selectCols = append(selectCols, `COUNT(*) FILTER (WHERE d.wides = 0) AS "ballsFaced"`)
		case "ballsBowled":
			selectCols = append(selectCols, `COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0) AS "ballsBowled"`)
		case "economyRate":
			selectCols = append(selectCols, `CASE WHEN COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0) > 0 THEN ROUND((SUM(d.runs_off_bat + d.wides + d.noballs)::numeric / (COUNT(*) FILTER (WHERE d.wides = 0 AND d.noballs = 0)::numeric / 6)), 2) ELSE 0 END AS "economyRate"`)
		case "strikeRate":
			selectCols = append(selectCols, `CASE WHEN COUNT(*) FILTER (WHERE d.wides = 0) > 0 THEN ROUND((SUM(d.runs_off_bat)::numeric / COUNT(*) FILTER (WHERE d.wides = 0)) * 100, 2) ELSE 0 END AS "strikeRate"`)
		default:
			selectCols = append(selectCols, fmt.Sprintf(`0 AS %s`, quoteIdentifier(metric)))
		}
	}

	defaultOrderBy := `ORDER BY m.start_date DESC`
	orderBy := buildStatExplorerOrderByClause(request.Sort, sortColumns, defaultOrderBy)
	offset := (request.Pagination.Page - 1) * request.Pagination.PageSize

	sql := fmt.Sprintf(`
		SELECT %s
		FROM wpl_delivery d
		JOIN wpl_match m ON d.match_id = m.match_id
		LEFT JOIN wpl_match_info mi ON m.match_id = mi.match_id
		WHERE %s
		GROUP BY %s
		%s
		LIMIT %d OFFSET %d
	`, strings.Join(selectCols, ",\n"), whereClause, strings.Join(groupByCols, ", "), orderBy, request.Pagination.PageSize, offset)

	countSQL := fmt.Sprintf(`
		SELECT COUNT(*)::int AS total
		FROM (
			SELECT 1
			FROM wpl_delivery d
			JOIN wpl_match m ON d.match_id = m.match_id
			LEFT JOIN wpl_match_info mi ON m.match_id = mi.match_id
			WHERE %s
			GROUP BY %s
		) grouped
	`, whereClause, strings.Join(groupByCols, ", "))

	return statExplorerBuiltQuery{sql: sql, countSQL: countSQL, args: builder.args, metricKeys: metricKeys}, nil
}

func buildStatExplorerWhereClause(
	builder *statExplorerSQLBuilder,
	reportType string,
	filters models.StatExplorerRunFilters,
	league string,
) string {
	conditions := []string{fmt.Sprintf("m.league = %s", builder.addArg(league)), "d.innings <= 2"}

	if len(filters.Teams) > 0 {
		stdTeams := standardizeTeams(filters.Teams)
		if reportType == "bowling" {
			conditions = append(conditions, fmt.Sprintf("d.bowling_team IN %s", builder.stringInClause(stdTeams)))
		} else {
			conditions = append(conditions, fmt.Sprintf("%s IN %s", standardizedBattingTeamSQL, builder.stringInClause(stdTeams)))
		}
	}
	if len(filters.Opposition) > 0 {
		stdOpposition := standardizeTeams(filters.Opposition)
		if reportType == "bowling" {
			conditions = append(conditions, fmt.Sprintf("%s IN %s", standardizedBattingTeamSQL, builder.stringInClause(stdOpposition)))
		} else {
			conditions = append(conditions, fmt.Sprintf("d.bowling_team IN %s", builder.stringInClause(stdOpposition)))
		}
	}
	if len(filters.Seasons) > 0 {
		conditions = append(conditions, fmt.Sprintf("m.season IN %s", builder.stringInClause(filters.Seasons)))
	}
	if filters.DateFrom != nil {
		conditions = append(conditions, fmt.Sprintf("m.start_date >= %s", builder.addArg(*filters.DateFrom)))
	}
	if filters.DateTo != nil {
		conditions = append(conditions, fmt.Sprintf("m.start_date <= %s", builder.addArg(*filters.DateTo)))
	}
	if len(filters.Venues) > 0 {
		conditions = append(conditions, fmt.Sprintf("m.venue IN %s", builder.stringInClause(filters.Venues)))
	}
	if len(filters.Cities) > 0 {
		conditions = append(conditions, fmt.Sprintf("mi.city IN %s", builder.stringInClause(filters.Cities)))
	}
	if len(filters.TossWinners) > 0 {
		conditions = append(conditions, fmt.Sprintf("mi.toss_winner IN %s", builder.stringInClause(standardizeTeams(filters.TossWinners))))
	}
	if len(filters.TossDecisions) > 0 {
		conditions = append(conditions, fmt.Sprintf("mi.toss_decision IN %s", builder.stringInClause(filters.TossDecisions)))
	}
	if len(filters.Innings) > 0 {
		conditions = append(conditions, fmt.Sprintf("d.innings IN %s", builder.intInClause(filters.Innings)))
	}
	if filters.OverFrom != nil && filters.OverTo != nil {
		conditions = append(conditions, fmt.Sprintf("CAST(SPLIT_PART(d.ball, '.', 1) AS int) BETWEEN %s AND %s", builder.addArg(*filters.OverFrom), builder.addArg(*filters.OverTo)))
	} else if filters.OverFrom != nil {
		conditions = append(conditions, fmt.Sprintf("CAST(SPLIT_PART(d.ball, '.', 1) AS int) >= %s", builder.addArg(*filters.OverFrom)))
	} else if filters.OverTo != nil {
		conditions = append(conditions, fmt.Sprintf("CAST(SPLIT_PART(d.ball, '.', 1) AS int) <= %s", builder.addArg(*filters.OverTo)))
	}
	if filters.Phase != nil && *filters.Phase != "overall" {
		phaseRanges := map[string][2]int{"powerplay": {1, 6}, "middle": {7, 15}, "death": {16, 20}}
		if value, ok := phaseRanges[*filters.Phase]; ok {
			conditions = append(conditions, fmt.Sprintf("CAST(SPLIT_PART(d.ball, '.', 1) AS int) BETWEEN %d AND %d", value[0], value[1]))
		}
	}
	if filters.ResultFilter != nil {
		teamSQL := standardizedBattingTeamSQL
		if reportType == "bowling" {
			teamSQL = "d.bowling_team"
		}
		switch *filters.ResultFilter {
		case "won":
			conditions = append(conditions, fmt.Sprintf("mi.winner IS NOT NULL AND %s = mi.winner", teamSQL))
		case "lost":
			conditions = append(conditions, fmt.Sprintf("mi.winner IS NOT NULL AND %s != mi.winner", teamSQL))
		case "noresult":
			conditions = append(conditions, "mi.winner IS NULL")
		}
	}

	return strings.Join(conditions, " AND ")
}

func buildStatExplorerOrderByClause(sort *models.StatExplorerSort, allowed map[string]string, defaultOrderBy string) string {
	if sort == nil {
		return defaultOrderBy
	}
	normalizedKey := strings.ToLower(sort.Key)
	if normalizedKey == "player" || normalizedKey == "team" {
		return defaultOrderBy
	}
	expr, ok := allowed[normalizedKey]
	if !ok {
		return defaultOrderBy
	}
	direction := "DESC"
	if strings.EqualFold(sort.Direction, "asc") {
		direction = "ASC"
	}
	return fmt.Sprintf("ORDER BY %s %s", expr, direction)
}

func statExplorerMetricSQL(metric string, isBowling bool) string {
	if isBowling {
		mapValue := map[string]string{
			"wickets":           "SUM(stats.wickets)",
			"ballsBowled":       "SUM(stats.balls_bowled)",
			"runsConceded":      "SUM(stats.runs_conceded)",
			"innings":           "COUNT(stats.match_id)",
			"economyRate":       "CASE WHEN SUM(stats.balls_bowled) > 0 THEN ROUND((SUM(stats.runs_conceded)::numeric / (SUM(stats.balls_bowled)::numeric / 6)), 2) ELSE 0 END",
			"bowlingAverage":    "CASE WHEN SUM(stats.wickets) > 0 THEN ROUND(SUM(stats.runs_conceded)::numeric / SUM(stats.wickets), 2) ELSE 0 END",
			"bowlingStrikeRate": "CASE WHEN SUM(stats.wickets) > 0 THEN ROUND(SUM(stats.balls_bowled)::numeric / SUM(stats.wickets), 2) ELSE 0 END",
			"fourWickets":       "COUNT(*) FILTER (WHERE stats.wickets >= 4 AND stats.wickets < 5)",
			"fiveWickets":       "COUNT(*) FILTER (WHERE stats.wickets >= 5)",
			"dotBalls":          "SUM(stats.dot_balls)",
			"matchesPlayed":     "COUNT(DISTINCT stats.match_id)",
			"matches":           "COUNT(DISTINCT stats.match_id)",
		}
		if value, ok := mapValue[metric]; ok {
			return value
		}
		return "0"
	}

	mapValue := map[string]string{
		"runs":          "SUM(stats.runs)",
		"ballsFaced":    "SUM(stats.balls_faced)",
		"innings":       "COUNT(stats.match_id)",
		"notOuts":       "SUM(1 - stats.is_dismissed)",
		"highestScore":  "MAX(stats.runs)",
		"fours":         "SUM(stats.fours)",
		"sixes":         "SUM(stats.sixes)",
		"fifties":       "COUNT(*) FILTER (WHERE stats.runs >= 50 AND stats.runs < 100)",
		"hundreds":      "COUNT(*) FILTER (WHERE stats.runs >= 100)",
		"strikeRate":    "CASE WHEN SUM(stats.balls_faced) > 0 THEN ROUND((SUM(stats.runs)::numeric / SUM(stats.balls_faced)) * 100, 2) ELSE 0 END",
		"average":       "CASE WHEN SUM(stats.is_dismissed) > 0 THEN ROUND(SUM(stats.runs)::numeric / SUM(stats.is_dismissed), 2) ELSE SUM(stats.runs)::numeric END",
		"dismissals":    "SUM(stats.is_dismissed)",
		"dotBalls":      "SUM(stats.dot_balls)",
		"matchesPlayed": "COUNT(DISTINCT stats.match_id)",
		"matches":       "COUNT(DISTINCT stats.match_id)",
	}
	if value, ok := mapValue[metric]; ok {
		return value
	}
	return "0"
}

func statExplorerSortColumnForDimension(dimension string) string {
	switch dimension {
	case "season", "player", "team", "opposition", "venue", "city", "result", "innings":
		return strings.ToLower(dimension)
	case "tossWinner":
		return "tosswinner"
	case "tossDecision":
		return "tossdecision"
	case "date":
		return "date_col"
	default:
		return quoteIdentifier(dimension)
	}
}

func normalizeStatExplorerValue(value interface{}, isNumeric bool) interface{} {
	switch typed := value.(type) {
	case nil:
		return nil
	case []byte:
		return normalizeStatExplorerStringValue(string(typed), isNumeric)
	case string:
		return normalizeStatExplorerStringValue(typed, isNumeric)
	case time.Time:
		return typed
	case int64, int32, int16, int8, int, float64, float32:
		if isNumeric {
			return fmt.Sprintf("%v", typed)
		}
		return typed
	case bool:
		return typed
	default:
		return fmt.Sprintf("%v", typed)
	}
}

func normalizeStatExplorerStringValue(value string, isNumeric bool) interface{} {
	if isNumeric {
		return value
	}
	return value
}

func quoteIdentifier(identifier string) string {
	return `"` + strings.ReplaceAll(identifier, `"`, `""`) + `"`
}

func standardizeTeams(teams []string) []string {
	standardized := make([]string, 0, len(teams))
	for _, team := range teams {
		standardized = append(standardized, standardizeTeamName(team))
	}
	return standardized
}

func standardizeTeamName(team string) string {
	switch team {
	case "Royal Challengers Bengaluru":
		return "Royal Challengers Bangalore"
	case "Delhi Daredevils":
		return "Delhi Capitals"
	case "Kings XI Punjab":
		return "Punjab Kings"
	case "Rising Pune Supergiants":
		return "Rising Pune Supergiant"
	default:
		return team
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

var _ = sql.ErrNoRows
