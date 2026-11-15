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
	go test -tags=integration -race -count=1 ./internal/repositories/...

db-up:
	docker run -d --name storybook-pg -p 5432:5432 -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=storybook postgres:16

lint:
	go vet ./...
	golangci-lint run ./...

build:
	go build -o bin/app .

# ponytail: skips with a notice when detect-secrets is not installed; CI always runs it.
secrets:
	@if command -v detect-secrets >/dev/null 2>&1; then \
		git ls-files -z | xargs -0 detect-secrets-hook --; \
	else echo "secrets: detect-secrets not installed, skipped locally (CI runs it)"; fi

done: lint test build secrets
	@echo "make done: OK"
