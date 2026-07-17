-- name: UpsertUserByFirebaseUID :one
INSERT INTO users (firebase_uid, email, display_name, avatar_url)
VALUES ($1, $2, $3, $4)
ON CONFLICT (firebase_uid) DO UPDATE SET
    email        = EXCLUDED.email,
    display_name = EXCLUDED.display_name,
    avatar_url   = EXCLUDED.avatar_url,
    updated_at   = now()
RETURNING *;

-- name: GetUserByFirebaseUID :one
SELECT * FROM users WHERE firebase_uid = $1;
