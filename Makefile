.PHONY: proto proto-lint build test run run-staging run-prod lint sqlc \
	migrate-up migrate-down migrate-create \
	docker-up docker-down integration-test \
	agentic-test

# Environment — override with TARGET_ENV=staging or TARGET_ENV=prod
TARGET_ENV ?= local

# Proto generation
proto:
	buf generate
	buf lint

proto-lint:
	buf lint

# Build
build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

# Run server — loads env/.env.$(TARGET_ENV) automatically
run:
	TARGET_ENV=$(TARGET_ENV) go run ./cmd/server

run-staging:
	TARGET_ENV=staging go run ./cmd/server

run-prod:
	TARGET_ENV=prod go run ./cmd/server

lint:
	golangci-lint run ./...

# sqlc
sqlc:
	sqlc generate

# Docker
docker-up:
	docker compose up -d

docker-down:
	docker compose down

# Database migrations
migrate-up:
	go run ./cmd/migrate -direction up

migrate-down:
	go run ./cmd/migrate -direction down -steps 1

migrate-create:
	@read -p "Migration name: " name; \
	cd db/migrations && \
	touch $$(date +%Y%m%d%H%M%S)_$$name.up.sql && \
	touch $$(date +%Y%m%d%H%M%S)_$$name.down.sql && \
	echo "Created migration files for $$name"

# Integration tests
integration-test:
	docker compose -f .devcontainer/docker-compose.yml up -d --wait
	DATABASE_URL=postgres://toqui:toqui@localhost:5433/toqui?sslmode=disable \
	FIRESTORE_EMULATOR_HOST=localhost:8081 \
	FIRESTORE_PROJECT_ID=toqui-test \
	go test -tags=integration -count=1 -v ./internal/integration/...
	docker compose -f .devcontainer/docker-compose.yml down

# Agentic tests — 20 Claude agents test the running backend via grpcurl.
# Requires: docker-up, migrate-up, server running on :8090.
# See tests/agentic/README.md and .claude/skills/agentic-test/SKILL.md.
agentic-test:
	@echo "Agentic tests are run via Claude Code orchestration."
	@echo "1. make docker-up && make migrate-up && make run"
	@echo "2. In Claude Code: run the agentic test suite"
	@echo "See tests/agentic/README.md for details."
