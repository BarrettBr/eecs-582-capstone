-- name: CreateUser :one
INSERT INTO users (id, username)
VALUES (?, ?)
RETURNING *;


-- name: DeleteUsers :exec
DELETE FROM users;


-- name: LookupUser :one
SELECT *
FROM users
WHERE username = ?;

-- name: LookupUserByID :one
SELECT *
FROM users
WHERE id = ?;
