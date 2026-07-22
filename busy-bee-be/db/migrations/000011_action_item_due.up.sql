-- 行動項到期日與提醒狀態：due_at 供提醒/行事曆，reminded_at 防重複推播（比對 meetings.reminded_at）。
ALTER TABLE action_items ADD COLUMN due_at timestamptz;
ALTER TABLE action_items ADD COLUMN reminded_at timestamptz;

-- 掃描到期未提醒的行動項用（部分索引，只涵蓋待提醒列）。
CREATE INDEX action_items_due_reminder_idx ON action_items (due_at)
    WHERE done = false AND reminded_at IS NULL AND due_at IS NOT NULL;
