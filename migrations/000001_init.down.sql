-- DropForeignKey
ALTER TABLE "wpl_delivery" DROP CONSTRAINT "wpl_delivery_match_id_fkey";

-- DropIndex
DROP INDEX "wpl_delivery_match_id_innings_ball_idx";

-- DropIndex
DROP INDEX "wpl_delivery_match_id_idx";

-- DropTable
DROP TABLE "wpl_delivery";

-- DropTable
DROP TABLE "wpl_match";
