-- AlterTable
-- Links each slug to a Cricsheet person (wpl_person_registry.registry_id), so a
-- profile URL stays with the same player when names are made unique or
-- Cricsheet renames someone. Nullable: rows are linked by the parser's
-- sync-names command, and new slugs are created with it set.
ALTER TABLE "player_slug" ADD COLUMN "registry_id" TEXT;

-- CreateIndex
CREATE UNIQUE INDEX "player_slug_registry_id_key" ON "player_slug"("registry_id");
