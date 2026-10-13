APP_ENV        ?= development
HTTP_ADDR      ?= :8090
UNIVERSE_CODE  ?= universe_gwx_1
UNIVERSE_NAME  ?= Classic Universe
GAME_SPEED     ?= 1
FLEET_SPEED    ?= 1
MAX_GALAXIES   ?= 3
MAX_SYSTEMS    ?= 199
BUILD_QUEUE_SLOTS ?= 6
RESEARCH_QUEUE_SLOTS ?= 4
NPC_BATCH      ?= 25
ENDS_AT        ?=
DATABASE_URL   ?= postgres://postgres:password@localhost:5433/gwx?sslmode=disable

export APP_ENV HTTP_ADDR UNIVERSE_CODE UNIVERSE_NAME GAME_SPEED FLEET_SPEED MAX_GALAXIES MAX_SYSTEMS BUILD_QUEUE_SLOTS RESEARCH_QUEUE_SLOTS NPC_BATCH ENDS_AT DATABASE_URL

.PHONY: run build test lint migrate up down

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

lint:
	go vet ./...

# Applies pending migrations through the ledger-aware runner (internal/store),
# which records each file in schema_migrations and skips it on later runs. Safe
# to re-run against a populated database. Pass a different dir: `make migrate ARGS=path`.
migrate:
	go run ./cmd/migrate $(ARGS)

up:
	docker compose up -d

down:
	docker compose down
