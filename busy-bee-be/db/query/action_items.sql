-- name: InsertActionItem :one
INSERT INTO action_items (meeting_id, user_id, description, assignee, due_text, sort_order)
VALUES ($1, $2, $3, $4, $5, $6)
RETURNING *;

-- name: DeleteActionItemsForMeeting :exec
DELETE FROM action_items WHERE meeting_id = $1;

-- name: ListActionItemsByMeeting :many
SELECT * FROM action_items WHERE meeting_id = $1 ORDER BY sort_order, created_at;

-- name: ListPendingActionItemsForUser :many
SELECT ai.*, m.title AS meeting_title
FROM action_items ai
JOIN meetings m ON m.id = ai.meeting_id
WHERE ai.user_id = $1 AND ai.done = false
ORDER BY ai.created_at DESC
LIMIT 100;

-- name: SetActionItemDone :one
UPDATE action_items
SET done = $3, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;
