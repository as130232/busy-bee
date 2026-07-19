CREATE TABLE push_subscriptions (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    endpoint   text NOT NULL UNIQUE,
    p256dh_key text NOT NULL,
    auth_key   text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);

-- 掃描式提醒的防重複標記（ADR-010：無延遲佇列，Sweeper 週期掃描）
ALTER TABLE meetings ADD COLUMN reminded_at timestamptz;

CREATE INDEX meetings_reminder_due_idx
    ON meetings (scheduled_at)
    WHERE status = 'scheduled' AND reminded_at IS NULL;
