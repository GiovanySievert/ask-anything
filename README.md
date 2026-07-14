# ask-anything

An AI-powered chat API in Go. Ingest technical documents, then hold multi-turn
conversations whose answers are grounded in that material (RAG) and streamed back
token by token over Server-Sent Events.

The pipeline: **ingest → chunk → embed → store** (documents), then per message
**embed → semantic search → Claude (streaming)** with the full conversation
history and the retrieved chunks.

## Stack

| Concern       | Choice                                             |
| ------------- | -------------------------------------------------- |
| Routing       | `net/http` + [chi](https://github.com/go-chi/chi)  |
| Database      | PostgreSQL + [pgvector](https://github.com/pgvector/pgvector) |
| DB access     | [pgx/v5](https://github.com/jackc/pgx) + [sqlc](https://sqlc.dev) |
| Embeddings    | [Ollama](https://ollama.com) (`nomic-embed-text`, 768 dims) — runs locally |
| LLM           | [Claude](https://www.anthropic.com) via the official Go SDK (streaming) |
| Streaming     | Server-Sent Events (`text/event-stream`)           |
| Validation    | [validator/v10](https://github.com/go-playground/validator) |
| Logging       | `log/slog` (structured)                            |
| Tests         | stdlib + testify (`httptest` for handlers)         |

**What runs where:** document text is chunked and embedded **locally** by Ollama
— it never leaves your machine during embedding. Only the chunks the semantic
search retrieves are sent to Claude to ground the reply.

## Prerequisites

- Go 1.26+
- Docker (for Postgres/pgvector and Ollama)
- An Anthropic API key with credits — https://console.anthropic.com

## Getting started

```bash
# 1. Configure environment
cp .env.example .env
# then edit .env and set ANTHROPIC_API_KEY=sk-ant-...

# 2. Start Postgres (pgvector) and Ollama
make db-up          # or: docker compose up -d db ollama

# 3. Pull the embedding model (one time)
docker exec ask_anything_ollama ollama pull nomic-embed-text

# 4. Apply migrations
make migrate-up

# 5. Run the API
make run
```

The API listens on `http://localhost:8080`.

```bash
curl http://localhost:8080/healthz          # {"status":"ok"}
```

## API

Base path: `/api/v1`

### Documents

| Method | Path         | Body                  | Success | Notes                          |
| ------ | ------------ | --------------------- | ------- | ------------------------------ |
| POST   | `/documents` | `{"title","content"}` | 201     | Chunks + embeds + stores it    |
| GET    | `/documents` | —                     | 200     | Lists ingested documents       |

```bash
curl -X POST http://localhost:8080/api/v1/documents \
  -H "Content-Type: application/json" \
  -d '{"title":"React Native FlatList","content":"FlatList renders large lists... use getItemLayout, keyExtractor, windowSize, FlashList..."}'
```

### Chat

| Method | Path                            | Body           | Success | Response                                  |
| ------ | ------------------------------- | -------------- | ------- | ----------------------------------------- |
| POST   | `/conversations`                | `{"title?"}`   | 201     | The created conversation                  |
| GET    | `/conversations`                | —              | 200     | Conversations, most recently updated first |
| GET    | `/conversations/{id}/messages`  | —              | 200     | The full message history (oldest first)   |
| POST   | `/conversations/{id}/messages`  | `{"content"}`  | 200     | **SSE stream** of the assistant reply     |

```bash
# 1. Start a conversation
CONV=$(curl -sX POST http://localhost:8080/api/v1/conversations \
  -d '{"title":"react native"}' | jq -r .id)

# 2. Send a message and stream the reply (RAG runs on every turn)
curl -N -X POST "http://localhost:8080/api/v1/conversations/$CONV/messages" \
  -H "Content-Type: application/json" \
  -d '{"content":"How would you optimize a slow FlatList?"}'

# 3. Read the persisted history
curl -s "http://localhost:8080/api/v1/conversations/$CONV/messages"
```

The `POST /conversations/{id}/messages` response is a Server-Sent Events stream.
It emits one `delta` event per text chunk, then a terminal `done` event carrying
the persisted assistant message; an `error` event is emitted if generation fails:

```
event: delta
data: {"text":"To optimize a slow "}

event: delta
data: {"text":"FlatList, use getItemLayout..."}

event: done
data: {"id":"...","conversation_id":"...","role":"assistant","content":"...","created_at":"..."}
```

Both the user message and the full assistant reply are persisted, so the next
turn is sent to Claude with the whole conversation history.

Errors on the JSON routes use a single envelope; validation errors add a
`fields` map:

```json
{ "error": { "message": "validation failed", "fields": { "Content": "failed on rule: required" } } }
```

## How RAG works here

```
INGESTION (POST /documents, once per doc):
  text ──chunk──> chunks ──Ollama──> vectors ──> stored in Postgres (pgvector)
                                        │
                              vectors are the search index, not sent to Claude

CHAT (POST /conversations/{id}/messages, per turn):
  user message ──Ollama──> vector
                             │
      pgvector finds the nearest chunks (embedding <=> vector, cosine distance)
                             │
   Claude receives those chunks (as the system prompt) + the full history
                             │
                 the reply, streamed back over SSE
```

The vector never reaches Claude — it's only used to find *which* chunks are
relevant. Claude sees the retrieved chunk text plus the conversation history.

## Project layout

```
cmd/api/            # entrypoint: wires config → db → clients → server
internal/
  config/           # env-var configuration
  server/           # http.Server, router, middleware, graceful shutdown
  chunking/         # pure Split(text, size, overlap) function
  embedding/        # Ollama client: text -> []float32
  llm/              # Claude client: StreamChat (streaming, multi-turn)
  document/         # the documents resource (domain → repository → service → handler)
  chat/             # the chat resource: conversations + messages + SSE streaming
  database/         # pgx pool + healthcheck
    db/             # sqlc-generated code (DO NOT EDIT)
  httputil/         # JSON read/write + standard error shape
db/
  migrations/       # *.up.sql / *.down.sql (000002 adds pgvector + documents/chunks, 000003 adds conversations/messages)
  queries/          # SQL consumed by sqlc (including the similarity search)
```

Each feature follows the same one-way flow: `handler → service → repository`,
with each layer depending on the next through an interface — so business logic
is testable without a database or network calls.

The chat handler splits its routes so the SSE endpoint runs without the
request-timeout middleware (which would otherwise cancel a long stream), and
clears the connection's write deadline via `http.NewResponseController`.

## Configuration

| Env var             | Required | Default                    |
| ------------------- | -------- | -------------------------- |
| `DATABASE_URL`      | yes      | —                          |
| `ANTHROPIC_API_KEY` | yes      | —                          |
| `PORT`              | no       | `8080`                     |
| `ENV`               | no       | `development`              |
| `LLM_MODEL`         | no       | `claude-haiku-4-5`         |
| `OLLAMA_URL`        | no       | `http://localhost:11434`   |
| `EMBEDDING_MODEL`   | no       | `nomic-embed-text`         |

`claude-haiku-4-5` is the default for cheap iteration. Swap `LLM_MODEL` for
`claude-opus-4-8` when you want the strongest model.

## API docs (Swagger)

Interactive OpenAPI docs are served at `http://localhost:8080/swagger/` while the
API is running. Regenerate them after changing handler annotations:

```bash
make docs          # swag init from the godoc annotations on the handlers
```

## Testing

```bash
make test          # everything, including integration tests
make test-short    # fast tests only (skips Docker/API-key-dependent tests)
```

- `internal/chunking` — pure unit tests.
- `internal/chat` — service unit tests (with fakes) + handler tests via `httptest`, including the SSE stream.
- `internal/embedding` — integration tests against a running Ollama (skipped in short mode).
- `internal/llm` — `StreamChat` integration test against the real Claude API (skipped unless `ANTHROPIC_API_KEY` is set).

## Useful commands

```bash
make help          # list all targets
make sqlc          # regenerate DB code after changing db/queries or migrations
make lint          # go vet + gofmt check
docker build .     # build the production image (multi-stage, distroless)
```

## Security notes

- `.env` holds your `ANTHROPIC_API_KEY` and is gitignored — never commit it, and
  never paste the key into logs, chat, or PRs. If it leaks, revoke it in the
  Anthropic console and generate a new one.
- Retrieved chunks are sent to the Anthropic API. For non-example / confidential
  material in production, review Anthropic's commercial terms / DPA and consider
  Zero Data Retention. API inputs are not used to train the models.

## Next steps (not included yet)

Auth/JWT, per-user conversations, auto-generated conversation titles, pagination,
re-ranking, PDF upload, and CI.
```
