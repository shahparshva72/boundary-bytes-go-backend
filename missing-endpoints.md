# Endpoint Migration Checklist

Based on a direct comparison between:
- `boundary-bytes/src/app/api/**/route.ts`
- `boundary-bytes-go-backend/internal/server/server.go`
- the Go handlers/database implementations used by the registered routes

## Summary

- Next.js exposes 26 method/path pairs across 24 route files.
- Go currently registers 22 matching API routes.
- 22 endpoints are now migrated in Go.
- 4 method/path pairs are still missing in Go.

## Recommended Sequential Order

### Phase 0: Finish Parity On What Already Exists

1. Fix `GET /api/stats/fall-of-wickets/{matchId}` route registration in `internal/server/server.go`.
2. Bring `GET /api/stats/leading-wicket-takers` to parity with Next.js wicket-crediting and runs-conceded logic.
3. Bring `GET /api/stats/leading-run-scorers` to parity by adding `fifties`, `hundreds`, and the same qualification/count logic.
4. Align common handler behavior where useful: default missing `league` to `WPL` and standardize JSON error responses.

Status: items 1-3 are complete.

### Phase 1: Add The Easy Read-Only Stats Endpoints

1. `GET /api/stats/latest-match-date`
2. `GET /api/stats/runrate-trend`
3. `GET /api/stats/team-runrate-progression`

Reason: these are read-only aggregate endpoints with limited branching and no new infrastructure.

### Phase 2: Add The Heavier Comparison Endpoint

1. `GET /api/stats/player-compare`

Reason: it is still a pure stats endpoint, but the query surface is much larger than Phase 1.

Status: complete.

### Phase 3: Add Stat Explorer Infrastructure

1. Apply the `player_style` table migration.
2. Load/backfill `player_style` data.
3. Implement `GET /api/stats/stat-explorer/options`.
4. Implement `POST /api/stats/stat-explorer/run`.

Reason: stat explorer is a subsystem, not just a single query. It depends on request validation, dynamic query building, and player-style lookups.

Status: stat explorer endpoints are complete. Full quality still depends on `player_style` data being backfilled in environments where style filters/dimensions are used.

### Phase 4: Add AI Feedback Endpoints

1. Verify `ai_chat_request` migration is applied in the Go database.
2. Implement `GET /api/ai/feedback`.
3. Implement `POST /api/ai/feedback`.

Reason: these are operationally simpler than text-to-SQL but still depend on persistent request logging.

### Phase 5: Add Text-to-SQL Last

1. Implement `OPTIONS /api/text-to-sql` and central CORS handling.
2. Port SQL validation and safe execution flow.
3. Port Gemini request/response handling.
4. Port request logging and response formatting.
5. Implement `POST /api/text-to-sql`.

Reason: this has the most external dependencies, security constraints, and moving pieces.

## Already Migrated In Go

- [x] `GET /api/players/batters`
- [x] `GET /api/players/bowlers`
- [x] `GET /api/matches/list`
- [x] `GET /api/matches`
- [x] `GET /api/news`
- [x] `GET /api/stats/seasons`
- [x] `GET /api/stats/latest-match-date`
- [x] `GET /api/stats/team-wins`
- [x] `GET /api/stats/team-averages`
- [x] `GET /api/stats/runrate-trend`
- [x] `GET /api/stats/team-runrate-progression`
- [x] `GET /api/stats/bowling-wicket-types`
- [x] `GET /api/stats/matchup`
- [x] `GET /api/stats/multi-matchup`
- [x] `GET /api/stats/player-compare`
- [x] `GET /api/stats/player-progression`
- [x] `GET /api/stats/stat-explorer/options`
- [x] `POST /api/stats/stat-explorer/run`
- [x] `GET /api/stats/advanced`
- [x] `GET /api/stats/fall-of-wickets/{matchId}`
- [x] `GET /api/stats/leading-wicket-takers`
- [x] `GET /api/stats/leading-run-scorers`

## Previously Unmigrated Endpoints

### Higher Effort, Now Migrated

- [x] `GET /api/ai/feedback`
  Requires AI request log storage and aggregate accuracy stats.

- [x] `POST /api/ai/feedback`
  Requires request lookup, feedback persistence, conflict handling, and structured validation.

- [x] `POST /api/text-to-sql`
  Largest migration item. Depends on Gemini SQL generation, SQL validation, safe query execution, result formatting, request logging, and team-result normalization.

- [x] `OPTIONS /api/text-to-sql`
  Needed for the current CORS behavior used by the Next.js route.

## SQL Migration Status

- [x] `ai_chat_request` migration already exists in Go as `000005_add_ai_chat_request_model.{up,down}.sql`
- [x] `player_style` migration was missing and has now been added as `000006_add_player_style_model.{up,down}.sql`

### Migration Follow-Up

- [ ] Apply `000005` if it has not been run in the target Go database yet.
- [ ] Apply `000006` before implementing full stat explorer parity.
- [ ] Backfill/import `player_style` records after `000006`; the migration only creates the table and indexes.

## Cross-Cutting Parity Notes

- Next.js defaults missing `league` to `WPL` via `validateLeague()`. The Go handlers currently require `league` explicitly, so even migrated endpoints do not yet match that default behavior.
- Next.js returns structured JSON validation errors on most routes. The Go handlers mostly use plain `http.Error`, so response shape is not yet consistent.
- If the frontend will call the Go service from a different origin, CORS still needs to be handled centrally in Go, not just on the AI endpoints.

## Source Files Checked

- Next.js route inventory: `boundary-bytes/src/app/api/**/route.ts`
- Go route registration: `boundary-bytes-go-backend/internal/server/server.go`
- Go handlers: `boundary-bytes-go-backend/internal/handlers/*.go`
- Go query implementations: `boundary-bytes-go-backend/internal/database/*.go`
