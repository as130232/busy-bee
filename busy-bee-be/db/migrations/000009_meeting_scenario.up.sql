-- 紀錄情境（scenario）：決定 AI 產出的結構化區塊模板；預設 meeting（會議），另有 casual（閒聊）。
-- summary_sections: [{"type":"decisions","title":"決議事項","items":["…","…"]}, …]
ALTER TABLE meetings
    ADD COLUMN scenario         text  NOT NULL DEFAULT 'meeting',
    ADD COLUMN summary_sections jsonb NOT NULL DEFAULT '[]';

ALTER TABLE meetings
    ADD CONSTRAINT meetings_scenario_check CHECK (scenario IN ('meeting', 'casual'));
