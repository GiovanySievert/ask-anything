# ask-anything

A production-shaped REST API in Go: layered architecture, PostgreSQL with
type-safe queries (sqlc), migrations, structured logging, graceful shutdown,
and integration tests against a real database.

It ships one example resource — `users` — as a full vertical slice you can copy
to build the next one.

## Stack

| Concern      | Choice                                   |
| ------------ | ---------------------------------------- |
| Routing      | `net/http` + [chi](https://github.com/go-chi/chi) |
| DB driver    | [pgx/v5](https://github.com/jackc/pgx)   |
| DB access    | [sqlc](https://sqlc.dev) (SQL → type-safe Go) |
| Migrations   | [golang-migrate](https://github.com/golang-migrate/migrate) (run via Docker) |
| Validation   | [validator/v10](https://github.com/go-playground/validator) |
| Logging      | `log/slog` (stdlib, structured)          |
| Tests        | stdlib + testify + [testcontainers](https://testcontainers.com) |

## Prerequisites

- Go 1.26+
- Docker (for Postgres and migrations)

## Getting started

```bash
# 1. Configure environment
cp .env.example .env

# 2. Start Postgres
make db-up

# 3. Apply migrations
make migrate-up

# 4. Run the API
make run          # or: make dev  (hot-reload, needs `air` installed)
```

The API listens on `http://localhost:8080`.

```bash
curl http://localhost:8080/healthz          # {"status":"ok"}
```

## API

Base path: `/api/v1`

| Method | Path           | Body                  | Success | Errors                    |
| ------ | -------------- | --------------------- | ------- | ------------------------- |
| POST   | `/users`       | `{"email","name"}`    | 201     | 400, 422 (invalid), 409 (dup email) |
| GET    | `/users`       | —                     | 200     |                           |
| GET    | `/users/{id}`  | —                     | 200     | 400 (bad id), 404         |
| PUT    | `/users/{id}`  | `{"email","name"}`    | 200     | 400, 404, 409, 422        |
| DELETE | `/users/{id}`  | —                     | 204     | 400, 404                  |

Query params on `GET /users`: `limit` (default 20, max 100), `offset`.

Errors use a single envelope:

```json
{ "error": { "message": "user not found" } }
```

Validation errors add a `fields` map:

```json
{ "error": { "message": "validation failed", "fields": { "Email": "failed on rule: email" } } }
```

### Example

```bash
curl -X POST http://localhost:8080/api/v1/users \
  -H "Content-Type: application/json" \
  -d '{"email":"ada@example.com","name":"Ada Lovelace"}'
```

## Project layout

```
cmd/api/            # entrypoint: wires config → db → server
internal/
  config/           # env-var configuration
  server/           # http.Server, router, middleware, graceful shutdown
  user/             # the users resource, one file per layer:
    user.go         #   domain models + domain errors
    repository.go   #   data access (maps sqlc rows/errors to the domain)
    service.go      #   business rules
    handler.go      #   HTTP: decode, validate, map errors to status codes
  database/         # pgx pool + healthcheck
    db/             # sqlc-generated code (DO NOT EDIT)
  httputil/         # JSON read/write + standard error shape
db/
  migrations/       # *.up.sql / *.down.sql
  queries/          # SQL consumed by sqlc
```

The dependency direction is one-way: `handler → service → repository → domain`.
Each layer talks to the next through an interface, so business logic is testable
without a database and HTTP details never leak into the core.

Anything under `internal/` cannot be imported by other Go modules — that is how
Go marks application-private packages.

## Changing the database

1. Create a migration: `make migrate-create name=add_something`
2. Edit the generated `*.up.sql` / `*.down.sql` in `db/migrations`.
3. Add/adjust queries in `db/queries/*.sql`.
4. Regenerate type-safe code: `make sqlc`
5. Apply: `make migrate-up`

## Testing

```bash
make test          # everything, including integration tests (needs Docker)
make test-short    # fast tests only (skips testcontainers)
```

Integration tests in `internal/user/repository_integration_test.go` spin up a
throwaway Postgres container, apply the migrations, and exercise the repository
against a real database.

## Useful commands

```bash
make help          # list all targets
make lint          # go vet + gofmt check
make build         # build ./bin/api
docker build .     # build the production image (multi-stage, distroless)
```

## Notes for developers coming from Node.js

- `context.Context` threads cancellation/timeouts through calls — like
  `AbortController`, but passed explicitly everywhere.
- Errors are values (`if err != nil`), not exceptions. Domain errors live in
  `user.go` and the handler maps them to HTTP status codes.
- Interfaces are satisfied implicitly. The service depends on the `Repository`
  interface; the Postgres implementation just happens to match it.
- `air` is the `nodemon` of Go (`make dev`).

## Next steps (not included yet)

Authentication/JWT, pagination metadata, rate limiting, OpenAPI docs, and CI.
Each is a natural addition on top of this base.
```
