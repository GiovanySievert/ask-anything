-- name: CreateConversation :one
INSERT INTO conversations (title)
VALUES ($1)
RETURNING *;

-- name: GetConversation :one
SELECT * FROM conversations
WHERE id = $1;

-- name: ListConversations :many
SELECT * FROM conversations
ORDER BY updated_at DESC
LIMIT $1 OFFSET $2;

-- name: TouchConversation :exec
UPDATE conversations
SET updated_at = now()
WHERE id = $1;

-- name: CreateMessage :one
INSERT INTO messages (conversation_id, role, content)
VALUES ($1, $2, $3)
RETURNING *;

-- name: ListMessages :many
SELECT * FROM messages
WHERE conversation_id = $1
ORDER BY created_at ASC;
