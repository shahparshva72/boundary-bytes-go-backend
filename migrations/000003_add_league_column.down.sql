-- DropIndex
DROP INDEX "wpl_match_info_league_season_idx";

-- DropIndex
DROP INDEX "wpl_match_info_league_idx";

-- DropIndex
DROP INDEX "wpl_match_league_season_idx";

-- DropIndex
DROP INDEX "wpl_match_league_idx";

-- AlterTable
ALTER TABLE "wpl_match_info" DROP COLUMN "league";

-- AlterTable
ALTER TABLE "wpl_match" DROP COLUMN "league";
