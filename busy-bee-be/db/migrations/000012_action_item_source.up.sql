-- source 區分行動項來源：llm（AI 抽取，重跑分析時整批重抽）/ manual（使用者手動新增，重跑不刪）。
ALTER TABLE action_items ADD COLUMN source text NOT NULL DEFAULT 'llm';
ALTER TABLE action_items ADD CONSTRAINT action_items_source_check
    CHECK (source IN ('llm', 'manual'));
