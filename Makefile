.PHONY: proto proto-lint build test run run-staging run-prod lint sqlc \
	migrate-up migrate-down migrate-create \
	docker-up docker-down integration-test \
	ai-test ai-test-generative

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

# AI integration tests — requires Docker infra + an AI provider key
# Uses real LLM calls to test the chat system end-to-end.
ai-test:
	FIRESTORE_EMULATOR_HOST=localhost:8080 \
	FIRESTORE_PROJECT_ID=toqui-test \
	go test -tags=aitest -count=1 -v -timeout=30m ./internal/aitest/...

# AI tests with LLM-generated exploratory scenarios
ai-test-generative:
	FIRESTORE_EMULATOR_HOST=localhost:8080 \
	FIRESTORE_PROJECT_ID=toqui-test \
	go test -tags=aitest -count=1 -v -timeout=30m ./internal/aitest/... -generative -gen-count=3
