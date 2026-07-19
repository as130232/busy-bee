DROP INDEX meetings_reminder_due_idx;
ALTER TABLE meetings DROP COLUMN reminded_at;
DROP TABLE push_subscriptions;
