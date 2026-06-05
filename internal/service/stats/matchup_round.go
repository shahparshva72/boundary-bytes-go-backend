package stats

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/models"
)

var ErrNoMatchupRound = errors.New("no valid matchup round found for league")

const maxMatchupRoundBatterAttempts = 12

type MatchupRoundResult struct {
	Batter          string
	Prompt          string
	QuestionType    string
	CorrectOpponent string
	Options         []models.MultiMatchupItem
	Leagues         []string
}

type matchupStrategy struct {
	questionType string
	prompt       func(string) string
	sortFunc     func(a, b models.MultiMatchupItem) bool
	isValid      func(sorted []models.MultiMatchupItem) bool
	metric       func(item models.MultiMatchupItem) float64
}

var matchupStrategies = []matchupStrategy{
	{
		questionType: "mostDismissals",
		prompt:       func(batter string) string { return fmt.Sprintf("%s — who has dismissed them the most?", batter) },
		sortFunc:     func(a, b models.MultiMatchupItem) bool { return a.Dismissals > b.Dismissals },
		isValid: func(sorted []models.MultiMatchupItem) bool {
			if len(sorted) < 3 {
				return false
			}
			return sorted[0].Dismissals > sorted[2].Dismissals
		},
		metric: func(item models.MultiMatchupItem) float64 { return float64(item.Dismissals) },
	},
	{
		questionType: "lowestStrikeRate",
		prompt:       func(batter string) string { return fmt.Sprintf("%s — who has the lowest strike rate against them?", batter) },
		sortFunc: func(a, b models.MultiMatchupItem) bool {
			if a.StrikeRate == b.StrikeRate {
				return a.Dismissals > b.Dismissals
			}
			return a.StrikeRate < b.StrikeRate
		},
		isValid: func(sorted []models.MultiMatchupItem) bool {
			withRate := filterItems(sorted, func(item models.MultiMatchupItem) bool { return item.StrikeRate > 0 })
			if len(withRate) < 3 {
				return false
			}
			return withRate[0].StrikeRate < withRate[2].StrikeRate
		},
		metric: func(item models.MultiMatchupItem) float64 { return item.StrikeRate },
	},
	{
		questionType: "fewestRuns",
		prompt:       func(batter string) string { return fmt.Sprintf("%s — who has conceded the fewest runs to them?", batter) },
		sortFunc: func(a, b models.MultiMatchupItem) bool {
			if a.RunsScored == b.RunsScored {
				return a.StrikeRate < b.StrikeRate
			}
			return a.RunsScored < b.RunsScored
		},
		isValid: func(sorted []models.MultiMatchupItem) bool {
			if len(sorted) < 3 {
				return false
			}
			return sorted[0].RunsScored < sorted[2].RunsScored
		},
		metric: func(item models.MultiMatchupItem) float64 { return float64(item.RunsScored) },
	},
}

func (s *Service) GetMatchupRound(ctx context.Context, league, seed string) (*MatchupRoundResult, error) {
	if seed == "" {
		seed = fmt.Sprintf("%d", time.Now().UnixNano())
	}

	batters, err := s.repository.GetEligibleMatchupBatters(ctx, league, seed)
	if err != nil {
		return nil, err
	}
	if len(batters) == 0 {
		return nil, ErrNoMatchupRound
	}

	attempts := maxMatchupRoundBatterAttempts
	if len(batters) < attempts {
		attempts = len(batters)
	}

	rng := rand.New(rand.NewSource(hashSeed(seed)))

	for i := 0; i < attempts; i++ {
		batter := batters[i]
		items, err := s.repository.GetBatterBowlersH2H(ctx, league, batter)
		if err != nil {
			return nil, err
		}
		if round := buildMatchupRoundFromItems(batter, items, rng); round != nil {
			round.Leagues = s.availableLeagues(ctx)
			return round, nil
		}
	}

	return nil, ErrNoMatchupRound
}

func buildMatchupRoundFromItems(batter string, items []models.MultiMatchupItem, rng *rand.Rand) *MatchupRoundResult {
	valid := filterItems(items, func(item models.MultiMatchupItem) bool {
		return item.BallsFaced >= 3 && (item.RunsScored > 0 || item.Dismissals > 0 || item.StrikeRate > 0)
	})
	if len(valid) < 3 {
		return nil
	}

	for _, strategy := range matchupStrategies {
		sorted := append([]models.MultiMatchupItem(nil), valid...)
		sort.Slice(sorted, func(i, j int) bool {
			return strategy.sortFunc(sorted[i], sorted[j])
		})
		if !strategy.isValid(sorted) {
			continue
		}

		topMetric := strategy.metric(sorted[0])
		var topCandidates []models.MultiMatchupItem
		for _, item := range sorted {
			if strategy.metric(item) == topMetric {
				topCandidates = append(topCandidates, item)
			}
		}
		correct := topCandidates[rng.Intn(len(topCandidates))]

		options := []models.MultiMatchupItem{correct}
		for _, item := range sorted {
			if len(options) >= 3 {
				break
			}
			if item.Opponent == correct.Opponent {
				continue
			}
			if containsOpponent(options, item.Opponent) {
				continue
			}
			options = append(options, item)
		}
		if len(options) < 3 {
			continue
		}

		rng.Shuffle(len(options), func(i, j int) {
			options[i], options[j] = options[j], options[i]
		})

		return &MatchupRoundResult{
			Batter:          batter,
			Prompt:          strategy.prompt(batter),
			QuestionType:    strategy.questionType,
			CorrectOpponent: correct.Opponent,
			Options:         options,
		}
	}

	return nil
}

func filterItems(items []models.MultiMatchupItem, predicate func(models.MultiMatchupItem) bool) []models.MultiMatchupItem {
	filtered := make([]models.MultiMatchupItem, 0, len(items))
	for _, item := range items {
		if predicate(item) {
			filtered = append(filtered, item)
		}
	}
	return filtered
}

func containsOpponent(items []models.MultiMatchupItem, opponent string) bool {
	for _, item := range items {
		if item.Opponent == opponent {
			return true
		}
	}
	return false
}

func hashSeed(seed string) int64 {
	var h int64
	for i := 0; i < len(seed); i++ {
		h = h*31 + int64(seed[i])
	}
	if h < 0 {
		h = -h
	}
	return h
}
