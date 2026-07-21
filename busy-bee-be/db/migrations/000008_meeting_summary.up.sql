-- 會議一句話摘要（TL;DR）：處理時由 LLM 於行動項抽取的同一次呼叫一併產生。
ALTER TABLE meetings ADD COLUMN summary text NOT NULL DEFAULT '';
