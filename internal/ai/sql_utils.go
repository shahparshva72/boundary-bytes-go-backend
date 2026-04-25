package ai

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

var playerLookupPattern = regexp.MustCompile(`(?i)select\s+(?:player_name|(?:ps\.)?name\s+as\s+player_name)\s+from\s+(?:wpl_player|player_style)\b`)

type ResolvedNames struct {
	BatterName string
	BowlerName string
}

type ExtractedQueries struct {
	BatterLookup string
	BowlerLookup string
	Main         string
}

func HasPlayerNameResolution(queries []string) bool {
	for _, query := range queries {
		if playerLookupPattern.MatchString(query) {
			return true
		}
	}
	return false
}

func ExtractHeadToHeadQueries(queries []string) (ExtractedQueries, error) {
	if len(queries) == 0 {
		return ExtractedQueries{}, errors.New("no queries provided")
	}

	extracted := ExtractedQueries{Main: queries[len(queries)-1]}
	lookups := []string{}
	for _, query := range queries[:len(queries)-1] {
		if playerLookupPattern.MatchString(query) {
			lookups = append(lookups, query)
		}
	}

	if len(lookups) >= 1 {
		extracted.BatterLookup = lookups[0]
	}
	if len(lookups) >= 2 {
		extracted.BowlerLookup = lookups[1]
	}

	return extracted, nil
}

func ApplyResolvedNames(query string, names ResolvedNames) string {
	result := query
	if names.BatterName != "" {
		quoted := SQLQuoteLiteral(names.BatterName)
		result = strings.ReplaceAll(result, "'RESOLVED_PLAYER_NAME'", quoted)
		result = strings.ReplaceAll(result, "RESOLVED_PLAYER_NAME", quoted)
		result = strings.ReplaceAll(result, "'RESOLVED_BATTER_NAME'", quoted)
		result = strings.ReplaceAll(result, "RESOLVED_BATTER_NAME", quoted)
	}
	if names.BowlerName != "" {
		quoted := SQLQuoteLiteral(names.BowlerName)
		result = strings.ReplaceAll(result, "'RESOLVED_BOWLER_NAME'", quoted)
		result = strings.ReplaceAll(result, "RESOLVED_BOWLER_NAME", quoted)
	}
	return result
}

func SQLQuoteLiteral(value string) string {
	return "'" + strings.ReplaceAll(value, "'", "''") + "'"
}

func ValidateSequentialQueries(queries []string) error {
	if len(queries) < 2 {
		return nil
	}

	first := strings.ToLower(queries[0])
	if playerLookupPattern.MatchString(first) && (!strings.Contains(first, "ilike") || !strings.Contains(first, "order by")) {
		return errors.New("player name resolution query is not properly structured")
	}

	for _, query := range queries {
		trimmed := strings.TrimSpace(strings.ToLower(query))
		if !(strings.HasPrefix(trimmed, "select") || strings.HasPrefix(trimmed, "with")) {
			return errors.New("all queries must be SELECT statements")
		}
	}

	return nil
}

func BuildExecutableQueries(queries []string, runQuery func(string) ([]map[string]interface{}, error)) ([]string, error) {
	if len(queries) == 0 {
		return nil, errors.New("no queries to build")
	}

	if !HasPlayerNameResolution(queries) {
		return queries, nil
	}

	extracted, err := ExtractHeadToHeadQueries(queries)
	if err != nil {
		return nil, err
	}

	names := ResolvedNames{}
	if extracted.BatterLookup != "" {
		rows, err := runQuery(extracted.BatterLookup)
		if err != nil {
			return nil, err
		}
		name, ok := firstPlayerName(rows)
		if !ok {
			return nil, errors.New("Player not found. Please check the name and try again.")
		}
		names.BatterName = name
	}

	if extracted.BowlerLookup != "" {
		rows, err := runQuery(extracted.BowlerLookup)
		if err != nil {
			return nil, err
		}
		name, ok := firstPlayerName(rows)
		if !ok {
			return nil, errors.New("No matching bowler found for lookup query")
		}
		names.BowlerName = name
	}

	finalQuery := ApplyResolvedNames(extracted.Main, names)
	normalized, err := MinimalValidateAndNormalize(finalQuery)
	if err != nil {
		return nil, err
	}

	return []string{normalized}, nil
}

func firstPlayerName(rows []map[string]interface{}) (string, bool) {
	if len(rows) == 0 {
		return "", false
	}

	for key, value := range rows[0] {
		if strings.EqualFold(key, "player_name") {
			name := strings.TrimSpace(toString(value))
			return name, name != ""
		}
	}

	return "", false
}

func toString(value interface{}) string {
	switch typed := value.(type) {
	case nil:
		return ""
	case string:
		return typed
	case []byte:
		return string(typed)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}
