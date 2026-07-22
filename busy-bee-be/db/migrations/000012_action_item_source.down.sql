ALTER TABLE action_items DROP CONSTRAINT IF EXISTS action_items_source_check;
ALTER TABLE action_items DROP COLUMN IF EXISTS source;
