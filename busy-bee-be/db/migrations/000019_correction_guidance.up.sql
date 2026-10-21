-- AI 校正前的使用者補充（錯字對照、補充說明）；隨校正保存，重試從斷點接續時重建相同輸入。只用於校正，不進報告。
ALTER TABLE transcript_corrections ADD COLUMN guidance jsonb NOT NULL DEFAULT '{}';
