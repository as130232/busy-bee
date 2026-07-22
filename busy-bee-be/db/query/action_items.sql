-- name: InsertActionItem :one
INSERT INTO action_items (meeting_id, user_id, description, assignee, due_text, due_at, source, sort_order)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
RETURNING *;

-- name: DeleteActionItemsForMeeting :exec
-- 只刪 LLM 抽取的行動項；手動新增（source='manual'）保留，重跑分析不受影響。
DELETE FROM action_items WHERE meeting_id = $1 AND source = 'llm';

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

-- name: UpdateActionItemDescription :one
UPDATE action_items
SET description = $3, updated_at = now()
WHERE id = $1 AND user_id = $2
RETURNING *;

-- name: ListDueActionItemReminders :many
SELECT ai.*, m.title AS meeting_title
FROM action_items ai
JOIN meetings m ON m.id = ai.meeting_id
WHERE ai.done = false
  AND ai.reminded_at IS NULL
  AND ai.due_at IS NOT NULL
  AND ai.due_at <= now()
  AND ai.due_at > now() - interval '2 days'
ORDER BY ai.due_at
LIMIT 50;

-- name: MarkActionItemReminded :exec
UPDATE action_items SET reminded_at = now(), updated_at = now() WHERE id = $1;
