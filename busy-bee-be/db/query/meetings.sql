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

-- name: GetMeeting :one
SELECT * FROM meetings WHERE id = $1;

-- name: SaveMeetingTranscript :one
UPDATE meetings
SET transcript = $2, transcript_segments = $3, duration_seconds = $4, updated_at = now()
WHERE id = $1
RETURNING *;

-- name: UpdateMeetingSpeakerNames :one
UPDATE meetings
SET speaker_names = sqlc.arg(speaker_names), updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: SetMeetingCompleted :one
UPDATE meetings
SET status = 'completed', processed_at = now(), error_message = '', updated_at = now()
WHERE id = $1 AND status = 'analyzing'
RETURNING *;

-- name: ListMeetingsForUser :many
SELECT * FROM meetings
WHERE user_id = $1
  AND (sqlc.arg(search)::text = ''
       OR title ILIKE '%' || sqlc.arg(search) || '%'
       OR transcript ILIKE '%' || sqlc.arg(search) || '%')
ORDER BY created_at DESC
LIMIT 100;

-- name: CreateScheduledMeeting :one
INSERT INTO meetings (user_id, title, status, scheduled_at, remind_before_min)
VALUES ($1, $2, 'scheduled', $3, $4)
RETURNING *;

-- name: UpdateMeetingSchedule :one
UPDATE meetings
SET title = $3, scheduled_at = $4, remind_before_min = $5, reminded_at = NULL, updated_at = now()
WHERE id = $1 AND user_id = $2 AND status = 'scheduled'
RETURNING *;

-- name: ListDueReminders :many
SELECT * FROM meetings
WHERE status = 'scheduled'
  AND reminded_at IS NULL
  AND scheduled_at IS NOT NULL
  AND scheduled_at - make_interval(mins => remind_before_min) <= now()
  AND scheduled_at > now() - interval '1 hour'
ORDER BY scheduled_at
LIMIT 50;

-- name: MarkMeetingReminded :exec
UPDATE meetings SET reminded_at = now(), updated_at = now() WHERE id = $1;

-- name: ListUnfinishedMeetingIDs :many
SELECT id FROM meetings
WHERE status IN ('pending', 'transcribing', 'analyzing')
ORDER BY created_at;

-- name: SetMeetingFailed :one
UPDATE meetings
SET status = 'failed', error_message = $2, updated_at = now()
WHERE id = $1 AND status IN ('pending', 'transcribing', 'analyzing')
RETURNING *;

-- name: RenameMeeting :one
UPDATE meetings
SET title = $3, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: DeleteMeeting :execrows
DELETE FROM meetings
WHERE id = $1 AND user_id = $2;
