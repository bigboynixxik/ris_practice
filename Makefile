.DEFAULT_GOAL := up

URL ?=

.PHONY: up down lint test test-integration demo-url demo-json db-clear db-status

up:
	docker compose up --build

down:
	docker compose down

lint:
	golangci-lint run

test:
	go test ./...

test-integration:
	go test -tags=integration -count=1 -timeout=5m ./internal/repository/postgres

demo-url:
	docker compose exec -e JSON_FILE= app /usr/local/bin/ris -once $(if $(URL),-url "$(URL)")

demo-json:
	docker compose exec app /usr/local/bin/ris -once -file /app/examples/laureates.json

db-clear:
	docker compose exec -T postgres psql -U ris -d ris -v ON_ERROR_STOP=1 -f /demo/clear.sql

db-status:
	docker compose exec -T postgres psql -U ris -d ris -v ON_ERROR_STOP=1 -f /demo/status.sql
