-- 匯入來源連結（YouTube/Podcast/直接音檔）；前端上傳的會議為空字串。
ALTER TABLE meetings ADD COLUMN source_url text NOT NULL DEFAULT '';
