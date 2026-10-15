-- 即時會議（Phase 30, F-LIVE）：live 狀態、會議中筆記、即時辨識用量；會前資料表；使用者偏好。
ALTER TABLE meetings DROP CONSTRAINT meetings_status_check;
ALTER TABLE meetings ADD CONSTRAINT meetings_status_check CHECK (
    status IN ('scheduled', 'live', 'pending', 'transcribing', 'analyzing', 'completed', 'failed')
);
ALTER TABLE meetings ADD COLUMN live_notes jsonb;
ALTER TABLE meetings ADD COLUMN live_seconds int NOT NULL DEFAULT 0;
ALTER TABLE meetings ADD COLUMN live_started_at timestamptz;
-- 月用量加總（user_id + 開始時間範圍）
CREATE INDEX meetings_user_live_started_idx ON meetings (user_id, live_started_at) WHERE live_started_at IS NOT NULL;

-- 會前資料：貼上文字或上傳檔案（轉成純文字），供即時重點、提問、正式摘要與 STT 術語使用。
CREATE TABLE meeting_contexts (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id uuid NOT NULL REFERENCES meetings (id) ON DELETE CASCADE,
    user_id    uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    title      text NOT NULL,
    source     text NOT NULL CHECK (source IN ('text', 'file')),
    gcs_path   text NOT NULL DEFAULT '',
    content    text NOT NULL,
    keyterms   text[] NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX meeting_contexts_meeting_idx ON meeting_contexts (meeting_id, created_at);

ALTER TABLE users ADD COLUMN live_mic_transcription boolean NOT NULL DEFAULT false;
