-- 手動標籤（分類/篩選用）；預設空陣列。
ALTER TABLE meetings ADD COLUMN tags text[] NOT NULL DEFAULT '{}';
