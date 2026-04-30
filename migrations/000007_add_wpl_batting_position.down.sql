-- DropTrigger
DROP TRIGGER "wpl_delivery_refresh_batting_positions_after_insert" ON "wpl_delivery";

-- DropFunction
DROP FUNCTION refresh_wpl_batting_positions_after_delivery_insert();

-- DropFunction
DROP FUNCTION refresh_wpl_batting_positions_for_match(INTEGER, BOOLEAN);

-- DropForeignKey
ALTER TABLE "wpl_batting_position" DROP CONSTRAINT "wpl_batting_position_first_delivery_id_fkey";

-- DropForeignKey
ALTER TABLE "wpl_batting_position" DROP CONSTRAINT "wpl_batting_position_match_id_fkey";

-- DropIndex
DROP INDEX "wpl_batting_position_first_delivery_id_idx";

-- DropIndex
DROP INDEX "wpl_batting_position_player_name_batting_position_idx";

-- DropIndex
DROP INDEX "wpl_batting_position_batting_position_idx";

-- DropIndex
DROP INDEX "wpl_batting_position_match_position_key";

-- DropIndex
DROP INDEX "wpl_batting_position_match_player_key";

-- DropTable
DROP TABLE "wpl_batting_position";
