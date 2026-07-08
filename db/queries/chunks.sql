-- name: CreateChunk :one
INSERT INTO chunks (document_id, content, embedding)
VALUES ($1, $2, $3)
RETURNING *;

-- name: SearchSimilarChunks :many
SELECT
    id,
    document_id,
    content,
    created_at,
    (embedding <=> $1)::float8 AS distance
FROM chunks
ORDER BY embedding <=> $1
LIMIT $2;
