.PHONY: proto proto-lint build test run run-staging run-prod lint sqlc \
	migrate-up migrate-down migrate-create \
	docker-up docker-down integration-test \
	agentic-test agentic-persona agentic-diff agentic-baseline agentic-validate \
	genguides

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

# Agentic tests — Claude agents test the running backend via buf curl.
# Requires: docker-up, migrate-up, server running on :8090.
# See tests/agentic/README.md and .claude/skills/agentic-test/SKILL.md.
agentic-test:
	@echo "Agentic tests are run via Claude Code orchestration."
	@echo "1. make docker-up && make migrate-up && make run"
	@echo "2. In Claude Code: run the agentic test suite"
	@echo "See tests/agentic/README.md for details."

# Print the instruction block for a single-persona replay. Copy the
# output into a Claude Code subagent Task to rerun one persona without
# the full orchestration batch. Requires PERSONA and TOKEN.
#
#   make agentic-persona PERSONA=R-02 TOKEN=eyJ... EMAIL=alice@...
agentic-persona:
	@test -n "$(PERSONA)" || (echo "PERSONA is required (e.g. PERSONA=R-02)" && exit 1)
	@test -n "$(TOKEN)" || (echo "TOKEN is required" && exit 1)
	@go run ./cmd/testctl run-persona \
		--id $(PERSONA) \
		--token "$(TOKEN)" \
		--host "$${HOST:-localhost:8090}" \
		$(if $(EMAIL),--expected-email "$(EMAIL)") \
		$(if $(RUN_ID),--run-id "$(RUN_ID)")

# Diff two run JSON files (or a bare array of reports, or a single
# report). Usage:
#
#   make agentic-diff FROM=run-5.json TO=run-6.json
agentic-diff:
	@test -n "$(FROM)" || (echo "FROM is required" && exit 1)
	@test -n "$(TO)" || (echo "TO is required" && exit 1)
	@go run ./cmd/testctl diff-runs --from "$(FROM)" --to "$(TO)"

# Compare a run against the committed baselines. Exits non-zero on any
# regression so CI can gate on this.
#
#   make agentic-baseline RUN=run-6.json
agentic-baseline:
	@test -n "$(RUN)" || (echo "RUN is required (file or --run-dir)" && exit 1)
	@go run ./cmd/testctl baseline-compare \
		--baselines tests/agentic/baselines \
		--run "$(RUN)"

# Validate a single agent report file against the report schema.
#
#   make agentic-validate FILE=tmp/r-02.json
agentic-validate:
	@test -n "$(FILE)" || (echo "FILE is required" && exit 1)
	@go run ./cmd/testctl validate-report --file "$(FILE)"

# Regenerate the curated 25-slug destination guide set from the persona
# system. PR 1 of toqui-backend#30 ships the tooling only — the live
# GuidesHandler still serves staticGuides() until PR 2 reviews the
# generated artefact and PR 3 flips the read path. Requires either
# ANTHROPIC_API_KEY or GEMINI_API_KEY in the environment. Writes the
# backend artefact (gitignored) and the toqui-site artefact (in the
# adjacent toqui-site checkout — committed separately, not in this PR).
#
#   make genguides
genguides:
	go run ./cmd/genguides \
		--output internal/handlers/guides_data.gen.json \
		--site-output ../toqui-site/src/data/guides.gen.ts
