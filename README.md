# ask-anything

An AI-powered technical-interview API in Go. Ingest technical documents, then
generate interview questions grounded in that material (RAG) and evaluate
candidate answers — all with structured JSON output.

The pipeline: **ingest → chunk → embed → store** (documents), then
**embed topic → semantic search → Claude** (questions) and **Claude** (answers).

## Stack

| Concern       | Choice                                             |
| ------------- | -------------------------------------------------- |
| Routing       | `net/http` + [chi](https://github.com/go-chi/chi)  |
| Database      | PostgreSQL + [pgvector](https://github.com/pgvector/pgvector) |
| DB access     | [pgx/v5](https://github.com/jackc/pgx) + [sqlc](https://sqlc.dev) |
| Embeddings    | [Ollama](https://ollama.com) (`nomic-embed-text`, 768 dims) — runs locally |
| LLM           | [Claude](https://www.anthropic.com) via the official Go SDK |
| Validation    | [validator/v10](https://github.com/go-playground/validator) |
| Logging       | `log/slog` (structured)                            |
| Tests         | stdlib + testify + [testcontainers](https://testcontainers.com) |

**What runs where:** document text is chunked and embedded **locally** by Ollama
— it never leaves your machine during embedding. Only the chunks the semantic
search retrieves are sent to Claude to generate a question.

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

### Interview

| Method | Path         | Body                    | Success | Response                       |
| ------ | ------------ | ----------------------- | ------- | ------------------------------ |
| POST   | `/questions` | `{"topic","level"}`     | 200     | `{"question"}`                 |
| POST   | `/answers`   | `{"question","answer"}` | 200     | `{"score","feedback","missing_points","weak_topics","next_question"}` |

```bash
# Generate a question (RAG: embeds the topic, finds similar chunks, asks Claude)
curl -X POST http://localhost:8080/api/v1/questions \
  -H "Content-Type: application/json" \
  -d '{"topic":"react native flatlist","level":"senior"}'

# Evaluate an answer
curl -X POST http://localhost:8080/api/v1/answers \
  -H "Content-Type: application/json" \
  -d '{"question":"How would you optimize a slow FlatList?","answer":"Use getItemLayout, memoize renderItem, switch to FlashList for huge lists."}'
```

Sample `/answers` response:

```json
{
  "score": 8,
  "feedback": "Strong, practical answer covering the most impactful optimizations.",
  "missing_points": ["removeClippedSubviews", "profiling before optimizing"],
  "weak_topics": ["performance measurement"],
  "next_question": "How would you determine whether the slowness is in rendering or the JS thread?"
}
```

Errors use a single envelope; validation errors add a `fields` map:

```json
{ "error": { "message": "validation failed", "fields": { "Level": "failed on rule: required" } } }
```

## How RAG works here

```
INGESTION (POST /documents, once per doc):
  text ──chunk──> chunks ──Ollama──> vectors ──> stored in Postgres (pgvector)
                                        │
                              vectors are the search index, not sent to Claude

QUESTION (POST /questions, per request):
  "react native senior" ──Ollama──> vector
                                       │
      pgvector finds the nearest chunks (embedding <=> vector, cosine distance)
                                       │
                    Claude receives those chunks (text) + the ask
                                       │
                              the generated question
```

The vector never reaches Claude — it's only used to find *which* chunks are
relevant. Claude sees the retrieved chunk text plus the instruction.

## Project layout

```
cmd/api/            # entrypoint: wires config → db → clients → server
internal/
  config/           # env-var configuration
  server/           # http.Server, router, middleware, graceful shutdown
  chunking/         # pure Split(text, size, overlap) function
  embedding/        # Ollama client: text -> []float32
  llm/              # Claude client: GenerateQuestion + EvaluateAnswer
  document/         # the documents resource (domain → repository → service → handler)
  interview/        # the RAG resource: /questions + /answers
  database/         # pgx pool + healthcheck
    db/             # sqlc-generated code (DO NOT EDIT)
  httputil/         # JSON read/write + standard error shape
db/
  migrations/       # *.up.sql / *.down.sql (000002 adds pgvector + documents/chunks)
  queries/          # SQL consumed by sqlc (including the similarity search)
```

Each feature follows the same one-way flow: `handler → service → repository`,
with each layer depending on the next through an interface — so business logic
is testable without a database or network calls.

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

## Testing

```bash
make test          # everything, including integration tests
make test-short    # fast tests only (skips Docker/API-key-dependent tests)
```

- `internal/chunking` — pure unit tests.
- `internal/embedding` — integration tests against a running Ollama (skipped in short mode).
- `internal/llm` — integration tests against the real Claude API (skipped unless `ANTHROPIC_API_KEY` is set).

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

Auth/JWT, pagination, streaming responses, re-ranking, PDF upload, and CI.
