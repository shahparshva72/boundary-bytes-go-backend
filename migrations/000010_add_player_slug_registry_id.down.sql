-- DropIndex
DROP INDEX IF EXISTS "player_slug_registry_id_key";

-- AlterTable
ALTER TABLE "player_slug" DROP COLUMN IF EXISTS "registry_id";
