-- CreateTable
-- Average scoring per ball for each league, season and phase. The API's
-- league-adjusted strike rate and economy divide a player's runs by these
-- rates. The phase expression must stay the same as phaseSQL() in
-- internal/repository/postgres/phase.go.
CREATE TABLE "league_phase_baseline" (
    "league" TEXT NOT NULL,
    "season" TEXT NOT NULL,
    "phase" TEXT NOT NULL,
    "balls_faced" INTEGER NOT NULL,
    "runs_off_bat" INTEGER NOT NULL,
    "legal_balls" INTEGER NOT NULL,
    "runs_conceded" INTEGER NOT NULL,
    "batting_runs_per_ball" NUMERIC,
    "bowling_runs_per_ball" NUMERIC,

    CONSTRAINT "league_phase_baseline_pkey" PRIMARY KEY ("league", "season", "phase")
);

-- CreateFunction
CREATE OR REPLACE FUNCTION refresh_league_phase_baseline()
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtext('league_phase_baseline'));

    DELETE FROM "league_phase_baseline";

    INSERT INTO "league_phase_baseline" (
        "league", "season", "phase",
        "balls_faced", "runs_off_bat", "legal_balls", "runs_conceded",
        "batting_runs_per_ball", "bowling_runs_per_ball"
    )
    SELECT
        b."league",
        b."season",
        b."phase",
        COUNT(*) FILTER (WHERE b."wides" = 0),
        SUM(b."runs_off_bat"),
        COUNT(*) FILTER (WHERE b."wides" = 0 AND b."noballs" = 0),
        SUM(b."runs_off_bat" + b."wides" + b."noballs"),
        SUM(b."runs_off_bat")::numeric / NULLIF(COUNT(*) FILTER (WHERE b."wides" = 0), 0),
        SUM(b."runs_off_bat" + b."wides" + b."noballs")::numeric
            / NULLIF(COUNT(*) FILTER (WHERE b."wides" = 0 AND b."noballs" = 0), 0)
    FROM (
        SELECT
            m."league",
            m."season",
            CASE
                WHEN CAST(SPLIT_PART(d."ball", '.', 1) AS int)
                    < (CASE WHEN COALESCE(mi."balls_per_over", 6) = 5 THEN 5 ELSE 6 END) THEN 'powerplay'
                WHEN CAST(SPLIT_PART(d."ball", '.', 1) AS int) >= 15 THEN 'death'
                ELSE 'middle'
            END AS "phase",
            d."runs_off_bat",
            d."wides",
            d."noballs"
        FROM "wpl_delivery" d
        JOIN "wpl_match" m ON d."match_id" = m."match_id"
        LEFT JOIN "wpl_match_info" mi ON mi."match_id" = d."match_id"
        WHERE d."innings" <= 2
    ) b
    GROUP BY b."league", b."season", b."phase";
END;
$$;

SELECT refresh_league_phase_baseline();
