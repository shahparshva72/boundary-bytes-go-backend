-- CreateTable
CREATE TABLE "wpl_batting_position" (
    "id" SERIAL NOT NULL,
    "match_id" INTEGER NOT NULL,
    "innings" INTEGER NOT NULL,
    "batting_team" TEXT NOT NULL,
    "player_name" TEXT NOT NULL,
    "batting_position" INTEGER NOT NULL,
    "first_delivery_id" INTEGER NOT NULL,

    CONSTRAINT "wpl_batting_position_pkey" PRIMARY KEY ("id"),
    CONSTRAINT "wpl_batting_position_batting_position_check" CHECK ("batting_position" BETWEEN 1 AND 11)
);

-- CreateFunction
CREATE OR REPLACE FUNCTION refresh_wpl_batting_positions_for_match(p_match_id INTEGER, p_take_lock BOOLEAN DEFAULT true)
RETURNS void
LANGUAGE plpgsql
AS $$
BEGIN
    IF p_take_lock THEN
        PERFORM pg_advisory_xact_lock(hashtext('wpl_batting_position'), p_match_id);
    END IF;

    DELETE FROM "wpl_batting_position"
    WHERE "match_id" = p_match_id;

    WITH batting_appearances AS (
        SELECT
            d."match_id",
            d."innings",
            d."batting_team",
            d."striker" AS "player_name",
            d."id" AS "delivery_id",
            COALESCE(NULLIF(SPLIT_PART(d."ball", '.', 1), '')::integer, 0) AS "over_number",
            COALESCE(NULLIF(SPLIT_PART(d."ball", '.', 2), '')::integer, 0) AS "ball_number",
            1 AS "role_order"
        FROM "wpl_delivery" d
        WHERE d."match_id" = p_match_id
            AND d."innings" <= 2

        UNION ALL

        SELECT
            d."match_id",
            d."innings",
            d."batting_team",
            d."non_striker" AS "player_name",
            d."id" AS "delivery_id",
            COALESCE(NULLIF(SPLIT_PART(d."ball", '.', 1), '')::integer, 0) AS "over_number",
            COALESCE(NULLIF(SPLIT_PART(d."ball", '.', 2), '')::integer, 0) AS "ball_number",
            2 AS "role_order"
        FROM "wpl_delivery" d
        WHERE d."match_id" = p_match_id
            AND d."innings" <= 2
    ),
    first_batting_appearances AS (
        SELECT DISTINCT ON ("match_id", "innings", "batting_team", "player_name")
            "match_id",
            "innings",
            "batting_team",
            "player_name",
            "delivery_id",
            "over_number",
            "ball_number",
            "role_order"
        FROM batting_appearances
        ORDER BY "match_id", "innings", "batting_team", "player_name", "over_number", "ball_number", "role_order", "delivery_id"
    ),
    batting_positions AS (
        SELECT
            "match_id",
            "innings",
            "batting_team",
            "player_name",
            "delivery_id",
            ROW_NUMBER() OVER (
                PARTITION BY "match_id", "innings", "batting_team"
                ORDER BY "over_number", "ball_number", "role_order", "delivery_id", "player_name"
            )::int AS "batting_position"
        FROM first_batting_appearances
    )
    INSERT INTO "wpl_batting_position" (
        "match_id",
        "innings",
        "batting_team",
        "player_name",
        "batting_position",
        "first_delivery_id"
    )
    SELECT
        "match_id",
        "innings",
        "batting_team",
        "player_name",
        "batting_position",
        "delivery_id"
    FROM batting_positions
    WHERE "batting_position" BETWEEN 1 AND 11;
END;
$$;

-- BackfillData
SELECT refresh_wpl_batting_positions_for_match("match_id", false)
FROM (
    SELECT DISTINCT "match_id"
    FROM "wpl_delivery"
    WHERE "innings" <= 2
) matches;

-- CreateIndex
CREATE UNIQUE INDEX "wpl_batting_position_match_player_key" ON "wpl_batting_position"("match_id", "innings", "batting_team", "player_name");

-- CreateIndex
CREATE UNIQUE INDEX "wpl_batting_position_match_position_key" ON "wpl_batting_position"("match_id", "innings", "batting_team", "batting_position");

-- CreateIndex
CREATE INDEX "wpl_batting_position_batting_position_idx" ON "wpl_batting_position"("batting_position");

-- CreateIndex
CREATE INDEX "wpl_batting_position_player_name_batting_position_idx" ON "wpl_batting_position"("player_name", "batting_position");

-- CreateIndex
CREATE INDEX "wpl_batting_position_first_delivery_id_idx" ON "wpl_batting_position"("first_delivery_id");

-- AddForeignKey
ALTER TABLE "wpl_batting_position" ADD CONSTRAINT "wpl_batting_position_match_id_fkey" FOREIGN KEY ("match_id") REFERENCES "wpl_match"("match_id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- AddForeignKey
ALTER TABLE "wpl_batting_position" ADD CONSTRAINT "wpl_batting_position_first_delivery_id_fkey" FOREIGN KEY ("first_delivery_id") REFERENCES "wpl_delivery"("id") ON DELETE RESTRICT ON UPDATE CASCADE;

-- CreateFunction
CREATE OR REPLACE FUNCTION refresh_wpl_batting_positions_after_delivery_insert()
RETURNS trigger
LANGUAGE plpgsql
AS $$
DECLARE
    inserted_match_id INTEGER;
BEGIN
    FOR inserted_match_id IN
        SELECT DISTINCT "match_id"
        FROM new_wpl_delivery_rows
        WHERE "innings" <= 2
    LOOP
        PERFORM refresh_wpl_batting_positions_for_match(inserted_match_id);
    END LOOP;

    RETURN NULL;
END;
$$;

-- CreateTrigger
CREATE TRIGGER "wpl_delivery_refresh_batting_positions_after_insert"
AFTER INSERT ON "wpl_delivery"
REFERENCING NEW TABLE AS new_wpl_delivery_rows
FOR EACH STATEMENT
EXECUTE FUNCTION refresh_wpl_batting_positions_after_delivery_insert();
