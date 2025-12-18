-- DropForeignKey
ALTER TABLE "wpl_person_registry" DROP CONSTRAINT "wpl_person_registry_match_id_fkey";

-- DropForeignKey
ALTER TABLE "wpl_official" DROP CONSTRAINT "wpl_official_match_id_fkey";

-- DropForeignKey
ALTER TABLE "wpl_player" DROP CONSTRAINT "wpl_player_match_id_fkey";

-- DropForeignKey
ALTER TABLE "wpl_team" DROP CONSTRAINT "wpl_team_match_id_fkey";

-- DropTable
DROP TABLE "wpl_person_registry";

-- DropTable
DROP TABLE "wpl_official";

-- DropTable
DROP TABLE "wpl_player";

-- DropTable
DROP TABLE "wpl_team";

-- DropTable
DROP TABLE "wpl_match_info";
