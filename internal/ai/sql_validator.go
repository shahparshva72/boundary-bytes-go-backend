package ai

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

type SQLValidationResult struct {
	IsValid  bool
	Errors   []string
	Warnings []string
}

var dangerousSQLKeywords = []string{
	"INSERT",
	"UPDATE",
	"DELETE",
	"DROP",
	"CREATE",
	"ALTER",
	"TRUNCATE",
	"EXEC",
	"EXECUTE",
	"DECLARE",
	"GRANT",
	"REVOKE",
	"COMMIT",
	"ROLLBACK",
	"SAVEPOINT",
	"MERGE",
	"CALL",
	"REPLACE",
	"LOAD",
	"COPY",
	"BULK",
	"BACKUP",
	"RESTORE",
	"ATTACH",
	"DETACH",
}

var allowedSQLTables = map[string]struct{}{
	"wpl_match":            {},
	"wpl_delivery":         {},
	"wpl_match_info":       {},
	"wpl_player":           {},
	"wpl_team":             {},
	"wpl_official":         {},
	"wpl_batting_position": {},
	"wpl_person_registry":  {},
	"player_style":         {},
}

var forbiddenSQLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)\binformation_schema\b`),
	regexp.MustCompile(`(?i)\bpg_[a-z0-9_]*\b`),
	regexp.MustCompile(`(?i)\bsys\.`),
	regexp.MustCompile(`(?i)\bmaster\.`),
	regexp.MustCompile(`(?i)\bmsdb\.`),
	regexp.MustCompile(`(?i)\btempdb\.`),
}

var injectionSQLPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i);\s*--`),
	regexp.MustCompile(`(?i);\s*/\*`),
	regexp.MustCompile(`(?i)\bunion\s+select\b`),
	regexp.MustCompile(`(?i)'\s*or\s*'1'\s*=\s*'1`),
	regexp.MustCompile(`(?i)'\s*or\s*1\s*=\s*1`),
	regexp.MustCompile(`(?i)\bxp_cmdshell\b`),
	regexp.MustCompile(`(?i)\bsp_executesql\b`),
	regexp.MustCompile(`(?i)\bpg_sleep\b`),
}

var (
	cteNamePattern  = regexp.MustCompile(`(?i)\b([A-Za-z_"][A-Za-z0-9_".]*)\s+AS\s*\(`)
	tableRefPattern = regexp.MustCompile(`(?i)\b(?:FROM|JOIN)\s+([A-Za-z_"][A-Za-z0-9_".]*)`)
	limitPattern    = regexp.MustCompile(`(?i)\bLIMIT\s+(\d+)`)
)

func ValidateSQL(sql string) SQLValidationResult {
	result := SQLValidationResult{Errors: []string{}, Warnings: []string{}}
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		result.Errors = append(result.Errors, "SQL query cannot be empty")
		result.IsValid = false
		return result
	}

	upper := strings.ToUpper(trimmed)
	if !(strings.HasPrefix(upper, "SELECT") || strings.HasPrefix(upper, "WITH")) {
		result.Errors = append(result.Errors, "Only SELECT queries are allowed")
	}

	if containsDangerousKeyword(trimmed) {
		result.Errors = append(result.Errors, "Query contains dangerous keywords. Only SELECT statements are allowed.")
	}

	if hasMultipleStatements(trimmed) {
		result.Errors = append(result.Errors, "Only a single SQL statement is allowed")
	}

	for _, pattern := range forbiddenSQLPatterns {
		if pattern.MatchString(trimmed) {
			result.Errors = append(result.Errors, "Access to system tables/schemas is not allowed")
			break
		}
	}

	for _, pattern := range injectionSQLPatterns {
		if pattern.MatchString(trimmed) {
			result.Errors = append(result.Errors, "Query contains potentially malicious patterns")
			break
		}
	}

	if errors := validateTableNames(trimmed); len(errors) > 0 {
		result.Errors = append(result.Errors, errors...)
	}

	result.IsValid = len(result.Errors) == 0
	return result
}

func MinimalValidateAndNormalize(sql string) (string, error) {
	trimmed := strings.TrimSpace(sql)
	if trimmed == "" {
		return "", fmt.Errorf("empty SQL from AI")
	}

	validation := ValidateSQL(trimmed)
	if !validation.IsValid {
		return "", errors.New(strings.Join(validation.Errors, ", "))
	}

	limitMatch := limitPattern.FindStringSubmatch(trimmed)
	if len(limitMatch) == 0 {
		return strings.TrimRight(trimmed, " ;") + " LIMIT 20", nil
	}

	limit, err := strconv.Atoi(limitMatch[1])
	if err == nil && limit > 20 {
		return limitPattern.ReplaceAllString(trimmed, "LIMIT 20"), nil
	}

	return trimmed, nil
}

func containsDangerousKeyword(sql string) bool {
	upper := strings.ToUpper(sql)
	for _, keyword := range dangerousSQLKeywords {
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(keyword) + `\b`).MatchString(upper) {
			return true
		}
	}
	return false
}

func hasMultipleStatements(sql string) bool {
	trimmed := strings.TrimSpace(sql)
	if !strings.Contains(trimmed, ";") {
		return false
	}
	withoutTrailing := strings.TrimSpace(strings.TrimSuffix(trimmed, ";"))
	return strings.Contains(withoutTrailing, ";")
}

func validateTableNames(sql string) []string {
	errors := []string{}
	cteNames := map[string]struct{}{}
	for _, match := range cteNamePattern.FindAllStringSubmatch(sql, -1) {
		if len(match) < 2 {
			continue
		}
		cteNames[cleanIdentifier(match[1])] = struct{}{}
	}

	for _, match := range tableRefPattern.FindAllStringSubmatch(sql, -1) {
		if len(match) < 2 {
			continue
		}
		tableName := cleanIdentifier(match[1])
		if _, ok := cteNames[tableName]; ok {
			continue
		}
		if _, ok := allowedSQLTables[tableName]; ok {
			continue
		}
		errors = append(errors, fmt.Sprintf("Table '%s' is not allowed. Only cricket tables are accessible.", match[1]))
	}

	return errors
}

func cleanIdentifier(identifier string) string {
	cleaned := strings.Trim(identifier, `"'`)
	if strings.Contains(cleaned, ".") {
		parts := strings.Split(cleaned, ".")
		cleaned = parts[len(parts)-1]
	}
	return strings.ToLower(strings.Trim(cleaned, `"'`))
}
