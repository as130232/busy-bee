-- name: CreateMeetingContext :one
INSERT INTO meeting_contexts (meeting_id, user_id, title, source, gcs_path, content, keyterms)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: ListMeetingContexts :many
SELECT * FROM meeting_contexts
WHERE meeting_id = $1 AND user_id = $2
ORDER BY created_at;

-- name: DeleteMeetingContext :one
DELETE FROM meeting_contexts
WHERE id = $1 AND meeting_id = $2 AND user_id = $3
RETURNING *;

-- name: SumMeetingContextChars :one
SELECT COALESCE(SUM(char_length(content)), 0)::bigint AS total
FROM meeting_contexts
WHERE meeting_id = $1 AND user_id = $2;
