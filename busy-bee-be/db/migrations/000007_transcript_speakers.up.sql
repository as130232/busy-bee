-- 分講者逐字稿（diarization）：片段陣列與講者代號→顯示名對應。
-- transcript_segments: [{"speaker":"A","text":"…","startMs":0,"endMs":1200}, …]
-- speaker_names:       {"A":"Ben", …}（限本場會議內，非跨會議身分）
ALTER TABLE meetings
    ADD COLUMN transcript_segments jsonb NOT NULL DEFAULT '[]',
    ADD COLUMN speaker_names       jsonb NOT NULL DEFAULT '{}';
