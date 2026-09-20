package texttosql

import (
	"context"
	"errors"
	"strings"
	"testing"

	aisql "github.com/shahparshva72/boundary-bytes-go-backend/internal/ai"
	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

func TestValidateQuestionRejectsInvalidCharacters(t *testing.T) {
	message := validateQuestion("top scorers <script>")
	if message == "" {
		t.Fatal("expected invalid characters to be rejected")
	}
}

func TestSanitizeQuestionNormalizesWhitespace(t *testing.T) {
	got := sanitizeQuestion("  top   scorers\nin WPL  ")
	want := "top scorers in WPL"
	if got != want {
		t.Fatalf("sanitizeQuestion() = %q, want %q", got, want)
	}
}

func TestNormalizeTeamResultsAggregatesCanonicalTeamNames(t *testing.T) {
	rows := []map[string]interface{}{
		{"team": "Royal Challengers Bengaluru", "wins": int64(2)},
		{"team": "Royal Challengers Bangalore", "wins": int64(3)},
	}

	got := normalizeTeamResults(rows)
	if len(got) != 1 {
		t.Fatalf("expected one normalized team row, got %d", len(got))
	}
	if got[0]["team"] != "Royal Challengers Bangalore" {
		t.Fatalf("team = %v, want Royal Challengers Bangalore", got[0]["team"])
	}
	if got[0]["wins"] != float64(5) {
		t.Fatalf("wins = %v, want 5", got[0]["wins"])
	}
}

type mockClassifier struct {
	classification *aisql.CricketClassification
	err            error
}

func (m *mockClassifier) ClassifyCricketQuery(_ context.Context, _ string) (*aisql.CricketClassification, error) {
	return m.classification, m.err
}

type mockGenerator struct {
	queries []string
	err     error
	called  bool
}

func (m *mockGenerator) GenerateSQL(_ context.Context, _ string) ([]string, error) {
	m.called = true
	return m.queries, m.err
}

type mockRepository struct {
	executedQuery string
	result        models.AIQueryResult
	err           error
}

func (m *mockRepository) ExecuteAIQuery(_ context.Context, query string) (models.AIQueryResult, error) {
	m.executedQuery = query
	return m.result, m.err
}

func (m *mockRepository) LogAIRequest(_ context.Context, _ models.LogAIRequestParams) (string, error) {
	return "req-test-123", nil
}

func TestAnswerRejectsNonCricketQueryWithJevGuardrail(t *testing.T) {
	repo := &mockRepository{}
	gen := &mockGenerator{queries: []string{"SELECT 1"}}
	classifier := &mockClassifier{
		classification: &aisql.CricketClassification{
			IsCricket:  false,
			Confidence: 0.95,
		},
	}

	svc := New(repo, gen).WithClassifier(classifier)
	_, err := svc.Answer(context.Background(), "What is the capital of France?")
	if err == nil {
		t.Fatal("expected error for non-cricket query, got nil")
	}

	aiErr, ok := err.(*Error)
	if !ok {
		t.Fatalf("expected *Error, got %T", err)
	}
	if aiErr.Code != CodeValidation {
		t.Fatalf("expected code VALIDATION_ERROR, got %s", aiErr.Code)
	}
	if gen.called {
		t.Fatal("expected Gemini generator to NOT be called when rejected by Jev guardrail")
	}
}

func TestAnswerFastRoutesWithJev(t *testing.T) {
	repo := &mockRepository{
		result: models.AIQueryResult{
			Data: []map[string]interface{}{
				{"striker": "Virat Kohli", "runs": int64(639)},
			},
			RowCount:        1,
			ExecutionTimeMS: 8,
		},
	}
	gen := &mockGenerator{queries: []string{"SELECT 1"}}
	classifier := &mockClassifier{
		classification: &aisql.CricketClassification{
			IsCricket:  true,
			Intent:     "leading_run_scorers",
			League:     "IPL",
			Confidence: 0.95,
		},
	}

	svc := New(repo, gen).WithClassifier(classifier)
	res, err := svc.Answer(context.Background(), "Top run scorers in IPL 2023")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.RoutedBy != "jev" {
		t.Fatalf("expected RoutedBy to be 'jev', got %q", res.RoutedBy)
	}
	if gen.called {
		t.Fatal("expected Gemini generator to NOT be called on fast-path route")
	}
	if len(res.Data) != 1 {
		t.Fatalf("expected 1 row, got %d", len(res.Data))
	}
}

func TestAnswerFallsBackToGeminiWhenJevClassifiesComplex(t *testing.T) {
	repo := &mockRepository{
		result: models.AIQueryResult{
			Data: []map[string]interface{}{
				{"player": "MS Dhoni", "runs": int64(50)},
			},
			RowCount:        1,
			ExecutionTimeMS: 12,
		},
	}
	gen := &mockGenerator{
		queries: []string{"SELECT d.striker, SUM(d.runs_off_bat) AS runs FROM wpl_delivery d WHERE d.innings <= 2 GROUP BY d.striker LIMIT 10"},
	}
	classifier := &mockClassifier{
		classification: &aisql.CricketClassification{
			IsCricket:  true,
			Intent:     "complex_query",
			Confidence: 0.70,
		},
	}

	svc := New(repo, gen).WithClassifier(classifier)
	res, err := svc.Answer(context.Background(), "Who scored more than 50 in death overs in finals?")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.RoutedBy != "gemini" {
		t.Fatalf("expected RoutedBy to be 'gemini', got %q", res.RoutedBy)
	}
	if !gen.called {
		t.Fatal("expected Gemini generator to be called for complex query")
	}
}

func TestBuildFastPathQueries(t *testing.T) {
	// 1. Leaderboards
	q1 := buildFastPathQueries("leading_run_scorers", "IPL", "all", "top run scorers in IPL 2023")
	if len(q1) != 1 {
		t.Fatalf("expected 1 query for leading_run_scorers, got %d", len(q1))
	}
	val1 := aisql.ValidateSQL(q1[0])
	if !val1.IsValid {
		t.Fatalf("invalid SQL for q1: %v", val1.Errors)
	}

	// 2. Bowling (economy rate calculation and dynamic limit)
	q2 := buildFastPathQueries("leading_wicket_takers", "WPL", "death", "Top 5 wicket takers in WPL")
	if len(q2) != 1 {
		t.Fatalf("expected 1 query for leading_wicket_takers, got %d", len(q2))
	}
	val2 := aisql.ValidateSQL(q2[0])
	if !val2.IsValid {
		t.Fatalf("invalid SQL for q2: %v", val2.Errors)
	}
	// Economy rate must calculate across ALL balls (not just wicket-taking balls)
	whereIdx := strings.LastIndex(q2[0], " WHERE ")
	groupByIdx := strings.Index(q2[0], " GROUP BY ")
	whereClause := q2[0][whereIdx:groupByIdx]
	if strings.Contains(whereClause, "player_dismissed") {
		t.Fatalf("expected main WHERE clause to NOT filter out non-wicket deliveries: %s", whereClause)
	}
	if !strings.Contains(q2[0], "LIMIT 5") {
		t.Fatalf("expected query to respect Top 5 limit, got: %s", q2[0])
	}

	// 3. Sixes
	q3 := buildFastPathQueries("most_sixes", "IPL", "all", "most sixes in IPL")
	if len(q3) != 1 {
		t.Fatalf("expected 1 query for most_sixes, got %d", len(q3))
	}

	// 4. Team wins
	q4 := buildFastPathQueries("team_record", "IPL", "all", "which team won the most matches")
	if len(q4) != 1 {
		t.Fatalf("expected 1 query for team_record, got %d", len(q4))
	}

	// 5. Single player batting (2 queries: lookup + main)
	q5 := buildFastPathQueries("player_batting_stat", "IPL", "all", "Virat Kohli runs in IPL 2023")
	if len(q5) != 2 {
		t.Fatalf("expected 2 queries for player_batting_stat, got %d", len(q5))
	}

	// 6. Head to head (3 queries: 2 lookups + main)
	q6 := buildFastPathQueries("head_to_head", "IPL", "all", "Virat Kohli vs Jasprit Bumrah")
	if len(q6) != 3 {
		t.Fatalf("expected 3 queries for head_to_head, got %d", len(q6))
	}
	if err := aisql.ValidateSequentialQueries(q6); err != nil {
		t.Fatalf("head to head sequential queries failed validation: %v", err)
	}

	// 7. Complex query returns nil for Gemini fallback
	qComplex := buildFastPathQueries("complex_query", "IPL", "all", "some complex question")
	if qComplex != nil {
		t.Fatalf("expected nil for complex_query to trigger Gemini fallback, got %v", qComplex)
	}

	// 8. Player query without player name returns nil for Gemini fallback
	qNoPlayer := buildFastPathQueries("player_batting_stat", "BBL", "all", "Highest strike rates in BBL")
	if qNoPlayer != nil {
		t.Fatalf("expected nil for player_batting_stat without named player, got %v", qNoPlayer)
	}
}

func TestExtractPlayerNameWithStopwords(t *testing.T) {
	full, surname := extractPlayerName("Highest strike rates in BBL")
	if full != "" || surname != "" {
		t.Fatalf("expected empty player name for 'Highest strike rates in BBL', got full=%q surname=%q", full, surname)
	}

	full, surname = extractPlayerName("Virat Kohli runs in IPL")
	if full != "Virat Kohli" || surname != "Kohli" {
		t.Fatalf("expected 'Virat Kohli' / 'Kohli', got full=%q surname=%q", full, surname)
	}
}

func TestAnswerFallsBackToGeminiWhenFastTemplateFails(t *testing.T) {
	// First query (fast-template) fails in repository, triggering fallback to Gemini
	callCount := 0
	repo := &mockRepositoryFunc{
		execFunc: func(ctx context.Context, query string) (models.AIQueryResult, error) {
			callCount++
			if callCount == 1 {
				return models.AIQueryResult{}, errors.New("template lookup error: relation does not exist")
			}
			return models.AIQueryResult{
				Data: []map[string]interface{}{
					{"striker": "Glenn Maxwell", "strike_rate": 160.5},
				},
				RowCount:        1,
				ExecutionTimeMS: 15,
			}, nil
		},
	}

	gen := &mockGenerator{
		queries: []string{"SELECT d.striker, ROUND(SUM(d.runs_off_bat)*100.0/COUNT(*), 2) AS strike_rate FROM wpl_delivery d GROUP BY d.striker LIMIT 10"},
	}

	classifier := &mockClassifier{
		classification: &aisql.CricketClassification{
			IsCricket:  true,
			Intent:     "leading_run_scorers",
			League:     "BBL",
			Confidence: 0.90,
		},
	}

	svc := New(repo, gen).WithClassifier(classifier)
	res, err := svc.Answer(context.Background(), "Top run scorers in BBL")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.RoutedBy != "gemini" {
		t.Fatalf("expected RoutedBy to be 'gemini' after fallback, got %q", res.RoutedBy)
	}
	if !gen.called {
		t.Fatal("expected Gemini generator to be called as fallback after fast template execution failure")
	}
}

type mockRepositoryFunc struct {
	execFunc func(ctx context.Context, query string) (models.AIQueryResult, error)
}

func (m *mockRepositoryFunc) ExecuteAIQuery(ctx context.Context, query string) (models.AIQueryResult, error) {
	if m.execFunc != nil {
		return m.execFunc(ctx, query)
	}
	return models.AIQueryResult{}, nil
}

func (m *mockRepositoryFunc) LogAIRequest(ctx context.Context, params models.LogAIRequestParams) (string, error) {
	return "log-1", nil
}

