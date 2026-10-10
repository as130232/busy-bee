-- name: InsertCorrection :one
INSERT INTO transcript_corrections (
    meeting_id, user_id, attempt, original_segments, original_transcript,
    original_summary_sections, original_summary)
VALUES ($1, $2, $3, $4, $5, $6, $7)
RETURNING *;

-- name: GetCorrection :one
SELECT * FROM transcript_corrections WHERE id = $1;

-- name: GetLatestCorrectionForMeeting :one
SELECT * FROM transcript_corrections
WHERE meeting_id = $1 AND user_id = $2
ORDER BY attempt DESC LIMIT 1;

-- name: CountCorrectionsForMeeting :one
SELECT count(*) FROM transcript_corrections WHERE meeting_id = $1 AND user_id = $2;

-- name: UpdateCorrectionStatus :one
UPDATE transcript_corrections
SET status = sqlc.arg(to_status)::text,
    error_message = sqlc.arg(error_message)::text,
    completed_at = CASE WHEN sqlc.arg(to_status)::text = 'completed' THEN now() ELSE completed_at END,
    reverted_at  = CASE WHEN sqlc.arg(to_status)::text = 'reverted'  THEN now() ELSE reverted_at END
WHERE id = sqlc.arg(id) AND status = sqlc.arg(from_status)::text
RETURNING *;

-- name: UpdateCorrectionProgress :exec
UPDATE transcript_corrections SET progress = $2 WHERE id = $1;

-- name: ListUnfinishedCorrectionIDs :many
SELECT id FROM transcript_corrections WHERE status IN ('queued', 'running');
