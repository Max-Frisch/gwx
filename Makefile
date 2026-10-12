APP_ENV        ?= development
HTTP_ADDR      ?= :8090
UNIVERSE_CODE  ?= universe_gwx_1
UNIVERSE_NAME  ?= Classic Universe
GAME_SPEED     ?= 1
RESOURCE_SPEED ?=
FLEET_SPEED    ?= 1
MAX_GALAXIES   ?= 3
MAX_SYSTEMS    ?= 199
ENDS_AT        ?=
DATABASE_URL   ?= postgres://postgres:password@localhost:5433/gwx?sslmode=disable

DB_CONTAINER ?= gwx_postgres
DB_USER      ?= postgres
DB_NAME      ?= gwx

export APP_ENV HTTP_ADDR UNIVERSE_CODE UNIVERSE_NAME GAME_SPEED RESOURCE_SPEED FLEET_SPEED MAX_GALAXIES MAX_SYSTEMS ENDS_AT DATABASE_URL

.PHONY: run build test lint migrate up down

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

lint:
	go vet ./...

# Applies every migration in order through the compose Postgres container
# (no local psql needed). Migrations are idempotent, so this is safe to re-run.
migrate:
	@for f in migrations/*.sql; do \
		echo "applying $$f"; \
		docker exec -i $(DB_CONTAINER) psql -U $(DB_USER) -d $(DB_NAME) -v ON_ERROR_STOP=1 < $$f || exit 1; \
	done

up:
	docker compose up -d

down:
	docker compose down
