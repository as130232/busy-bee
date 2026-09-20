-- STT 辨識語言（per-meeting）：繁中(zh-TW，預設)/英文(en-US)/自動偵測(auto)。
-- auto 走 Deepgram detect_language=true；偵測為簡體中文時由 infrastructure/stt 後製轉繁體。
ALTER TABLE meetings ADD COLUMN language text NOT NULL DEFAULT 'zh-TW';

ALTER TABLE meetings
    ADD CONSTRAINT meetings_language_check CHECK (language IN ('zh-TW', 'en-US', 'auto'));
