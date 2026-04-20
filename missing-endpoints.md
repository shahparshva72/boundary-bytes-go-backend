# Missing API Endpoints (boundary-bytes-go-backend)

This file lists API endpoints that exist in the Next.js boundary-bytes project but are not yet implemented in the Go backend. The list is ordered from easiest to hardest to implement based on query complexity, data dependencies, and external integrations.

Existing Go endpoints (for reference):
- GET /health
- GET /db-health
- GET /api/players/batters
- GET /api/players/bowlers
- GET /api/stats/matchup
- GET /api/stats/leading-wicket-takers
- GET /api/stats/leading-run-scorers
- GET /api/news

## Prioritized Missing Endpoints (Easiest -> Hardest)

| Priority | Endpoint | Method(s) | Notes on Functionality | Ease Rationale |
| --- | --- | --- | --- | --- |
| 1 | /api/stats/seasons | GET | Returns distinct seasons for a league from `wpl_match_info`. Requires `league` query param. | Single SELECT with DISTINCT and ORDER BY. |
| 2 | /api/matches/list | GET | Returns match list with team names, league, season, venue, date. Requires `league`. | One query with aggregation; minimal post-processing. |
| 3 | /api/matches | GET | Paginated match cards with scores, teams, results, seasons list. Requires `league`; supports `page`, `limit`, optional `season`. | Moderate SQL with CTEs plus pagination and computed result text. |
| 4 | /api/stats/team-wins | GET | Team wins/losses, batting first/second stats. Requires `league`. | Multi-CTE query but self-contained; no special dependencies. |
| 5 | /api/stats/team-averages | GET | Batting averages, strike rate, highest/lowest scores per team. Requires `league`. | Multi-CTE query with aggregates; straightforward to port. |
| 6 | /api/stats/bowling-wicket-types | GET | Wicket-type breakdown by bowler with pagination. Requires `league`; supports `page`, `limit`. | Aggregate query plus a total-count query; moderate complexity. |
| 7 | /api/stats/multi-matchup | GET | Stats for one player vs multiple opponents. Requires `league`, `player`, `opponents`, `mode`. | Single query with list param; minor result aggregation for combined totals. |
| 8 | /api/stats/player-progression | GET | Over-by-over progression for a batter (runs, SR, average). Requires `league`, `player`; optional `innings`. | Aggregate query plus derived per-over calculations. |
| 9 | /api/stats/advanced | GET | Advanced split stats for batter or bowler across overs. Requires `league`, `overs`, plus `batter` or `bowler`. | Fetches deliveries then computes metrics in memory; more logic. |
| 10 | /api/stats/fall-of-wickets/{matchId} | GET | Fall of wickets per innings for a match. Requires path `matchId` and `league`. | Multiple queries with joins and row-number calculations; more complex response shaping. |
| 11 | /api/stats/player-compare | GET | Compare 2-5 players with batting/bowling stats, filters for seasons/team/statType. Requires `league`, `players`; optional `seasons`, `team`, `statType`. | Multiple CTEs and conditional logic; largest SQL footprint. |
| 12 | /api/ai/feedback | GET, POST, OPTIONS | Stores AI response feedback and returns accuracy stats. Requires request logging storage. | Requires additional DB schema/services beyond stats queries. |
| 13 | /api/text-to-sql | POST, OPTIONS | AI-powered natural language to SQL with validation, execution, logging. | External AI integration, security validation, request logging, and query execution pipeline. |
