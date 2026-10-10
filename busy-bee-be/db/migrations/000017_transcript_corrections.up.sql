-- AI 錄音重新校正（Phase 27, F-CORRECT）：每次校正一筆，含校正前快照（還原用）與逐段斷點 progress。
CREATE TABLE transcript_corrections (
    id                        uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id                uuid NOT NULL REFERENCES meetings (id) ON DELETE CASCADE,
    user_id                   uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    attempt                   int  NOT NULL CHECK (attempt BETWEEN 1 AND 2),
    status                    text NOT NULL DEFAULT 'queued'
        CHECK (status IN ('queued', 'running', 'completed', 'failed', 'reverted')),
    progress                  jsonb NOT NULL DEFAULT '{"chunks":{}}',
    original_segments         jsonb NOT NULL,
    original_transcript       text  NOT NULL,
    original_summary_sections jsonb NOT NULL,
    original_summary          text  NOT NULL,
    error_message             text  NOT NULL DEFAULT '',
    created_at                timestamptz NOT NULL DEFAULT now(),
    completed_at              timestamptz,
    reverted_at               timestamptz,
    UNIQUE (meeting_id, attempt)
);
-- 同一場同時只能有一筆進行中的校正
CREATE UNIQUE INDEX transcript_corrections_active_idx
    ON transcript_corrections (meeting_id) WHERE status IN ('queued', 'running');
CREATE INDEX transcript_corrections_unfinished_idx
    ON transcript_corrections (status) WHERE status IN ('queued', 'running');

-- 行動項是否被使用者改過內容；AI 校正重抽時保留 edited 項。
ALTER TABLE action_items ADD COLUMN edited boolean NOT NULL DEFAULT false;
-- 舊資料保守回填：曾被更新過（改內容、勾選又取消、寄過提醒）一律視為改過，寧可多留不誤刪。
UPDATE action_items SET edited = true WHERE updated_at > created_at;
