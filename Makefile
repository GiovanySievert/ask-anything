ifneq (,$(wildcard .env))
include .env
export
endif

MIGRATIONS_DIR := db/migrations

MIGRATE_DATABASE_URL := $(subst @localhost:,@host.docker.internal:,$(DATABASE_URL))

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: up
up: ## Start everything (db + ollama + migrations + API) in one command
	docker compose up -d db ollama
	@echo "waiting for postgres to be healthy..."
	@until [ "$$(docker inspect -f '{{.State.Health.Status}}' ask_anything_db 2>/dev/null)" = "healthy" ]; do sleep 1; done
	@echo "ensuring embedding model is present..."
	@docker exec ask_anything_ollama ollama pull $(EMBEDDING_MODEL) >/dev/null 2>&1 || true
	$(MAKE) migrate-up
	@echo "starting API..."
	go run ./cmd/api

.PHONY: down
down: ## Stop all containers (keeps data)
	docker compose down

.PHONY: db-up
db-up: ## Start only the Postgres container
	docker compose up -d db

.PHONY: db-down
db-down: ## Stop the Postgres container
	docker compose down

.PHONY: run
run: ## Run only the API (assumes db + ollama already up)
	go run ./cmd/api

.PHONY: dev
dev:
	air

.PHONY: build
build:
	go build -o bin/api ./cmd/api

.PHONY: sqlc
sqlc:
	sqlc generate

.PHONY: docs
docs: ## Regenerate the OpenAPI/Swagger docs from handler annotations
	$(shell go env GOPATH)/bin/swag init -g cmd/api/main.go --parseDependency --parseInternal -o docs

.PHONY: migrate-up
migrate-up:
	docker run --rm -v $(PWD)/$(MIGRATIONS_DIR):/migrations --add-host=host.docker.internal:host-gateway \
		migrate/migrate -path=/migrations -database "$(MIGRATE_DATABASE_URL)" up

.PHONY: migrate-down
migrate-down:
	docker run --rm -v $(PWD)/$(MIGRATIONS_DIR):/migrations --add-host=host.docker.internal:host-gateway \
		migrate/migrate -path=/migrations -database "$(MIGRATE_DATABASE_URL)" down 1

.PHONY: migrate-create
migrate-create:
	docker run --rm -v $(PWD)/$(MIGRATIONS_DIR):/migrations \
		migrate/migrate create -ext sql -dir /migrations -seq $(name)

.PHONY: test
test:
	go test ./... -race -count=1

.PHONY: test-short
test-short:
	go test ./... -short -count=1

.PHONY: lint
lint:
	go vet ./...
	@test -z "$$(gofmt -l .)" || (echo "unformatted files:"; gofmt -l .; exit 1)

.PHONY: tidy
tidy:
	go mod tidy
