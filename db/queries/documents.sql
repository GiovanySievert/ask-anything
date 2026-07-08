-- name: CreateDocument :one
INSERT INTO documents (title)
VALUES ($1)
RETURNING *;

-- name: GetDocument :one
SELECT * FROM documents
WHERE id = $1;

-- name: ListDocuments :many
SELECT * FROM documents
ORDER BY created_at DESC
LIMIT $1 OFFSET $2;
