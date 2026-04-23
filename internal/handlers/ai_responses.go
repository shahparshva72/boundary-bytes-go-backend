package handlers

import (
	"regexp"
	"strings"
)

type textToSQLSuccessResponse struct {
	Success   bool                     `json:"success"`
	Data      []map[string]interface{} `json:"data"`
	Metadata  textToSQLMetadata        `json:"metadata"`
	RequestID string                   `json:"requestId,omitempty"`
}

type textToSQLMetadata struct {
	RowCount      int    `json:"rowCount"`
	ExecutionTime int    `json:"executionTime"`
	GeneratedSQL  string `json:"generatedSql"`
}

type apiErrorResponse struct {
	Success     bool     `json:"success"`
	Error       string   `json:"error"`
	Code        string   `json:"code"`
	Suggestions []string `json:"suggestions,omitempty"`
	Tips        []string `json:"tips,omitempty"`
}

func formatAIError(message, code string, context ...string) apiErrorResponse {
	sanitized := sanitizeErrorMessage(message)
	suggestions, tips := generateAISuggestions(code, sanitized, contextValue(context))
	return apiErrorResponse{
		Success:     false,
		Error:       sanitized,
		Code:        code,
		Suggestions: suggestions,
		Tips:        tips,
	}
}

func formatAIServerError() apiErrorResponse {
	return apiErrorResponse{
		Success: false,
		Error:   "An unexpected error occurred while processing your cricket question. Please try again.",
		Code:    "DATABASE_ERROR",
		Suggestions: []string{
			"Who are the top run scorers in WPL?",
		},
		Tips: []string{
			"Please try again in a moment",
			"If the problem persists, try asking a simpler cricket question",
		},
	}
}

func formatAITimeoutError(originalQuestion string) apiErrorResponse {
	tips := []string{
		"Try asking about fewer players or a shorter time period",
		"Break complex questions into simpler parts",
	}
	if originalQuestion != "" {
		tips = append(tips, `Original question: "`+originalQuestion+`"`)
	} else {
		tips = append(tips, "Try rephrasing your question")
	}

	return apiErrorResponse{
		Success:     false,
		Error:       "Your cricket question took too long to process. Please try a simpler question.",
		Code:        "DATABASE_ERROR",
		Suggestions: []string{"Top 5 run scorers in WPL 2023"},
		Tips:        tips,
	}
}

func formatAIUnavailableError() apiErrorResponse {
	return apiErrorResponse{
		Success: false,
		Error:   "The AI service is temporarily unavailable. Please try again in a moment.",
		Code:    "AI_ERROR",
		Tips: []string{
			"Please wait a moment and try again",
			"The AI service should be back online shortly",
			"You can try asking your cricket question again in a few minutes",
		},
	}
}

func sanitizeErrorMessage(message string) string {
	replacements := []struct {
		pattern *regexp.Regexp
		value   string
	}{
		{regexp.MustCompile(`(?i)database\s+connection`), "connection"},
		{regexp.MustCompile(`(?i)prisma`), "database"},
		{regexp.MustCompile(`(?i)postgresql`), "database"},
		{regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`), "[server]"},
		{regexp.MustCompile(`(?i)password`), "[credentials]"},
		{regexp.MustCompile(`(?i)token`), "[credentials]"},
	}

	sanitized := message
	for _, replacement := range replacements {
		sanitized = replacement.pattern.ReplaceAllString(sanitized, replacement.value)
	}
	return sanitized
}

func generateAISuggestions(code, errorMessage, context string) ([]string, []string) {
	suggestions := []string{}
	tips := []string{}

	switch code {
	case "VALIDATION_ERROR":
		tips = append(tips,
			"Make sure your question contains only letters, numbers, and basic punctuation",
			"Keep your question under 500 characters",
			"Try asking about cricket statistics like top run scorers or bowling figures",
		)
		suggestions = append(suggestions, "Top run scorers in WPL 2023", "Best bowling figures in WPL 2023")
	case "AI_ERROR":
		tips = append(tips,
			"Try rephrasing your cricket question more clearly",
			"Ask about specific players, teams, or statistics",
			"Be specific about the season or tournament such as WPL 2023 or IPL 2024",
		)
		suggestions = append(suggestions, "Who scored the most runs in WPL 2023?")
	case "SQL_ERROR":
		if strings.Contains(strings.ToLower(errorMessage), "player name") {
			tips = append(tips,
				"Try using just the last name such as Mandhana instead of Smriti Mandhana",
				"Check the spelling of player names",
				"Make sure the player exists in the WPL database",
			)
			suggestions = append(suggestions, "Smriti Mandhana's average in WPL 2023")
		} else {
			tips = append(tips,
				"Your question might be too complex, try breaking it into simpler parts",
				"Ask about one statistic at a time",
				"Make sure you are asking about cricket data that exists in the database",
			)
			suggestions = append(suggestions, "Top 5 strike rates in WPL 2023")
		}
	case "DATABASE_ERROR":
		lower := strings.ToLower(errorMessage)
		if strings.Contains(lower, "not found") || strings.Contains(lower, "no statistics") {
			tips = append(tips,
				"Check if the player name or team name is spelled correctly",
				"Try using partial names such as Kohli instead of Virat Kohli",
				"Ask about WPL players and teams specifically",
				"Try different seasons or tournaments that might have the data you are looking for",
			)
			suggestions = append(suggestions, "Team with most wins in WPL 2023")
		} else if strings.Contains(lower, "connection") || strings.Contains(lower, "timeout") {
			tips = append(tips,
				"There seems to be a temporary connection issue",
				"Please try again in a moment",
				"If the problem persists, try asking a simpler cricket question",
			)
		} else {
			tips = append(tips,
				"Please try again in a moment",
				"If the problem persists, try asking a simpler cricket question",
				"Make sure your question is about cricket statistics present in the database",
			)
			suggestions = append(suggestions, "Best bowling economy in WPL 2023")
		}
	case "RATE_LIMIT_ERROR":
		tips = append(tips,
			"Please wait a moment before asking another question",
			"You can ask up to 30 questions per minute",
			"Take your time to think about what cricket statistics you would like to explore",
		)
	}

	contextLower := strings.ToLower(context)
	if strings.Contains(contextLower, "player") && !strings.Contains(contextLower, "wpl") {
		tips = append(tips, "Make sure to ask about WPL players specifically")
	}
	if strings.Contains(contextLower, "team") && !strings.Contains(contextLower, "wpl") {
		tips = append(tips, "Try asking about WPL teams like Mumbai Indians or Royal Challengers Bangalore")
	}

	if len(suggestions) == 0 {
		tips = append(tips, "Try asking about cricket statistics like runs, wickets, or strike rates")
		suggestions = append(suggestions,
			"Top 5 batters in WPL",
			"Best bowling figures in WPL",
			"Which team won the most matches in WPL 2023?",
		)
	}

	return suggestions, tips
}

func contextValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
