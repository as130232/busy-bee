-- name: UpsertArtifact :one
INSERT INTO artifacts (meeting_id, artifact_type, content)
VALUES ($1, $2, $3)
ON CONFLICT (meeting_id, artifact_type) DO UPDATE SET content = EXCLUDED.content
RETURNING *;

-- name: ListArtifactsByMeeting :many
SELECT * FROM artifacts WHERE meeting_id = $1 ORDER BY artifact_type;
