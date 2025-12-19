.PHONY: run build migrate-up migrate-down migrate-create migrate-force migrate-version

# Database URL from environment or default
DATABASE_URL ?= $(shell echo $$DATABASE_URL)

run:
	go run cmd/api/main.go

watch:
	air

build:
	go build -o bin/api cmd/api/main.go

# Run all up migrations
migrate-up:
	migrate -path migrations -database "$(DATABASE_URL)" up

# Run a specific number of up migrations
migrate-up-n:
	migrate -path migrations -database "$(DATABASE_URL)" up $(N)

# Rollback all migrations
migrate-down:
	migrate -path migrations -database "$(DATABASE_URL)" down

# Rollback a specific number of migrations
migrate-down-n:
	migrate -path migrations -database "$(DATABASE_URL)" down $(N)

# Create a new migration file
migrate-create:
	migrate create -ext sql -dir migrations -seq $(NAME)

# Force set a migration version (use with caution)
migrate-force:
	migrate -path migrations -database "$(DATABASE_URL)" force $(VERSION)

# Show current migration version
migrate-version:
	migrate -path migrations -database "$(DATABASE_URL)" version

# Drop everything in the database
migrate-drop:
	migrate -path migrations -database "$(DATABASE_URL)" drop -f
