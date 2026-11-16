.PHONY: run migrate seed test test-integration lint build secrets done db-up

run:
	go run main.go run

migrate:
	go run main.go migrate up

seed:
	go run main.go seed

test:
	go test ./... -race -cover

# Needs Postgres (make db-up). Uses TEST_DB_DSN or the default below.
test-integration:
	go test -tags=integration -race -count=1 -p 1 ./internal/repositories/...

db-up:
	docker run -d --name storybook-pg -p 5433:5432 -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=storybook postgres:16
	sleep 3 && docker exec storybook-pg psql -U postgres -c "CREATE DATABASE storybook_test"

lint:
	go vet -tags=integration ./...

build:
	go build -o bin/app .

# ponytail: skips with a notice when detect-secrets is not installed.
secrets:
	@if command -v detect-secrets >/dev/null 2>&1; then \
		git ls-files -z | xargs -0 detect-secrets-hook --; \
	else echo "secrets: detect-secrets not installed, skipped"; fi

done: lint test build secrets
	@echo "make done: OK"
