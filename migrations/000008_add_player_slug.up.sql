-- CreateTable
CREATE TABLE "player_slug" (
    "slug" TEXT NOT NULL,
    "player_name" TEXT NOT NULL,
    "leagues" TEXT[] NOT NULL DEFAULT '{}',
    "updated_at" TIMESTAMP(3) NOT NULL DEFAULT now(),

    CONSTRAINT "player_slug_pkey" PRIMARY KEY ("slug")
);

-- CreateIndex
CREATE UNIQUE INDEX "player_slug_player_name_key" ON "player_slug"("player_name");
CREATE INDEX "player_slug_leagues_idx" ON "player_slug" USING GIN ("leagues");

-- Backfill: one row per distinct player (batter or bowler), across all leagues.
-- Slug collisions (two different names slugifying to the same value, e.g.
-- "S. Sharma" and "S Sharma") get a numeric suffix appended, deterministically
-- ordered by name so re-runs are stable.
WITH distinct_players AS (
    SELECT DISTINCT striker AS player_name FROM wpl_delivery
    UNION
    SELECT DISTINCT bowler AS player_name FROM wpl_delivery
),
player_leagues AS (
    SELECT p.player_name, array_agg(DISTINCT m.league ORDER BY m.league) AS leagues
    FROM distinct_players p
    JOIN wpl_delivery d ON d.striker = p.player_name OR d.bowler = p.player_name
    JOIN wpl_match m ON m.match_id = d.match_id
    GROUP BY p.player_name
),
base_slugs AS (
    SELECT
        player_name,
        leagues,
        lower(regexp_replace(regexp_replace(player_name, '[^a-zA-Z0-9]+', '-', 'g'), '(^-|-$)', '', 'g')) AS base_slug
    FROM player_leagues
),
numbered AS (
    SELECT
        player_name,
        leagues,
        base_slug,
        ROW_NUMBER() OVER (PARTITION BY base_slug ORDER BY player_name) AS rn
    FROM base_slugs
)
INSERT INTO player_slug (slug, player_name, leagues)
SELECT
    CASE WHEN rn = 1 THEN base_slug ELSE base_slug || '-' || rn END,
    player_name,
    leagues
FROM numbered;