-- name: CreateMeeting :one
INSERT INTO meetings (user_id, title, audio_gcs_path, status, scheduled_at, remind_before_min)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: GetMeetingForUser :one
SELECT * FROM meetings WHERE id = $1 AND user_id = $2;

-- name: UpdateMeetingStatus :one
UPDATE meetings
SET status = sqlc.arg(to_status), updated_at = now()
WHERE id = $1 AND status = sqlc.arg(from_status)
RETURNING *;
