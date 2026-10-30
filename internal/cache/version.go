package cache

import (
	"context"
	"database/sql"
	"log"
	"sync"
	"time"
)

// DataVersion identifies the loaded data. It is part of every cache key, so
// when the parser loads matches or renames players the version changes and
// old entries are never read again; they just expire. This keeps the parser
// free of any cache knowledge.
type DataVersion struct {
	db       *sql.DB
	interval time.Duration

	mu        sync.Mutex
	version   string
	checkedAt time.Time
}

func NewDataVersion(db *sql.DB, interval time.Duration) *DataVersion {
	return &DataVersion{db: db, interval: interval}
}

// dataVersionQuery changes when matches are added (count) and when the
// parser's player sync renames anyone or refreshes slugs (updated_at).
const dataVersionQuery = `
	SELECT
		(SELECT COUNT(*) FROM wpl_match)::text
		|| ':' ||
		COALESCE((SELECT EXTRACT(EPOCH FROM MAX(updated_at))::bigint FROM player_slug), 0)::text
`

// Current returns the version, re-reading it at most once per interval. If
// the read fails it keeps the last known version.
func (v *DataVersion) Current(ctx context.Context) string {
	v.mu.Lock()
	defer v.mu.Unlock()

	if v.version != "" && time.Since(v.checkedAt) < v.interval {
		return v.version
	}

	var version string
	if err := v.db.QueryRowContext(ctx, dataVersionQuery).Scan(&version); err != nil {
		log.Printf("cache: reading data version failed: %v", err)
		if v.version == "" {
			return "unknown"
		}
		return v.version
	}

	v.version = version
	v.checkedAt = time.Now()
	return v.version
}
