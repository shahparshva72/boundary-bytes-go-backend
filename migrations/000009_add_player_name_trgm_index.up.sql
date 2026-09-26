-- Speeds up the text-to-sql fast-path player lookups (player_name ILIKE '%surname%'),
-- which otherwise seq-scan every per-match player row.
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- CreateIndex
CREATE INDEX "wpl_player_player_name_trgm_idx" ON "wpl_player" USING GIN ("player_name" gin_trgm_ops);
