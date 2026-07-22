DROP INDEX IF EXISTS action_items_due_reminder_idx;
ALTER TABLE action_items DROP COLUMN IF EXISTS reminded_at;
ALTER TABLE action_items DROP COLUMN IF EXISTS due_at;
