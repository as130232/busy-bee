ALTER TABLE users DROP COLUMN live_mic_transcription;
DROP TABLE meeting_contexts;
DROP INDEX meetings_user_live_started_idx;
ALTER TABLE meetings DROP COLUMN live_started_at;
ALTER TABLE meetings DROP COLUMN live_seconds;
ALTER TABLE meetings DROP COLUMN live_notes;
UPDATE meetings SET status = 'scheduled' WHERE status = 'live';
ALTER TABLE meetings DROP CONSTRAINT meetings_status_check;
ALTER TABLE meetings ADD CONSTRAINT meetings_status_check CHECK (
    status IN ('scheduled', 'pending', 'transcribing', 'analyzing', 'completed', 'failed')
);
