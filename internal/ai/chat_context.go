package ai

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shahparshva72/boundary-bytes-go-backend/internal/leagues"
)

// SeasonLabel is a Cricsheet season that spans two calendar years, e.g.
// BBL "2023/24", with the year most of its matches were played in.
type SeasonLabel struct {
	Label string
	Year  int
}

// SeasonIndex lists each league's split-year seasons. Cricsheet labels some
// seasons by one year ("2024") and others by two ("2023/24"); the mapping
// isn't consistent across leagues (IPL 2010 is "2009/10", NPL 2024 is
// "2024/25"), so it is read from the data rather than written as rules.
type SeasonIndex map[string][]SeasonLabel

// LabelFor returns the season a league played mostly in year. When two
// seasons share a year it prefers the one ending in it ("BBL 2012" ->
// "2011/12"), the usual way such seasons are named.
func (s SeasonIndex) LabelFor(code string, year int) (string, bool) {
	var candidates []string
	for _, l := range s[code] {
		if l.Year == year {
			candidates = append(candidates, l.Label)
		}
	}
	if len(candidates) == 0 {
		return "", false
	}
	suffix := fmt.Sprintf("/%02d", year%100)
	for _, c := range candidates {
		if strings.HasSuffix(c, suffix) {
			return c, true
		}
	}
	return candidates[0], true
}

// PromptLines renders one line per league: "BBL: 2023/24 (2024), ...".
func (s SeasonIndex) PromptLines() []string {
	codes := make([]string, 0, len(s))
	for code := range s {
		codes = append(codes, code)
	}
	sort.Strings(codes)

	lines := make([]string, 0, len(codes))
	for _, code := range codes {
		parts := make([]string, 0, len(s[code]))
		for _, l := range s[code] {
			parts = append(parts, fmt.Sprintf("%s (%d)", l.Label, l.Year))
		}
		lines = append(lines, fmt.Sprintf("  %s: %s", code, strings.Join(parts, ", ")))
	}
	return lines
}

const seasonIndexQuery = `
	SELECT league, season,
		mode() WITHIN GROUP (ORDER BY EXTRACT(YEAR FROM start_date)::int)
	FROM wpl_match
	WHERE season LIKE '%/%'
	GROUP BY league, season
	ORDER BY league, season
`

func loadSeasonIndex(ctx context.Context, db *sql.DB) (SeasonIndex, error) {
	rows, err := db.QueryContext(ctx, seasonIndexQuery)
	if err != nil {
		return nil, fmt.Errorf("load season index: %w", err)
	}
	defer rows.Close()

	index := SeasonIndex{}
	for rows.Next() {
		var code string
		var label SeasonLabel
		if err := rows.Scan(&code, &label.Label, &label.Year); err != nil {
			return nil, fmt.Errorf("scan season index: %w", err)
		}
		index[code] = append(index[code], label)
	}
	return index, rows.Err()
}

// ChatContext supplies what text-to-SQL needs to know about the data: the
// visible leagues and their season labels. The season index is re-read at
// most once per refresh interval so newly loaded seasons show up.
type ChatContext struct {
	db       *sql.DB
	registry *leagues.Registry
	refresh  time.Duration

	mu       sync.Mutex
	seasons  SeasonIndex
	prompt   string
	loadedAt time.Time
}

func NewChatContext(db *sql.DB, registry *leagues.Registry, refresh time.Duration) *ChatContext {
	return &ChatContext{db: db, registry: registry, refresh: refresh}
}

func (c *ChatContext) Registry() *leagues.Registry {
	return c.registry
}

// Seasons returns the season index, keeping the last good copy if a reload
// fails.
func (c *ChatContext) Seasons(ctx context.Context) SeasonIndex {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reloadLocked(ctx)
	return c.seasons
}

// Prompt returns the Gemini system prompt for the current leagues and seasons.
func (c *ChatContext) Prompt(ctx context.Context) string {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.reloadLocked(ctx)
	return c.prompt
}

func (c *ChatContext) reloadLocked(ctx context.Context) {
	if c.prompt != "" && time.Since(c.loadedAt) < c.refresh {
		return
	}

	if c.db != nil {
		seasons, err := loadSeasonIndex(ctx, c.db)
		if err != nil {
			log.Printf("chat context: %v (keeping previous seasons)", err)
		} else {
			c.seasons = seasons
		}
	}
	c.prompt = BuildMasterPrompt(c.registry, c.seasons)
	c.loadedAt = time.Now()
}
