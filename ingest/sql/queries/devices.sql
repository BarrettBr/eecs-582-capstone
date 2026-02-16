-- name: CreateDevice :exec
INSERT INTO devices (id, name)
VALUES (?, ?);

-- name: GetDevice :one
SELECT id, name, created_at
FROM devices
WHERE id = ?;
