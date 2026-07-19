ALTER TABLE artifacts DROP CONSTRAINT artifacts_type_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_type_check
    CHECK (artifact_type IN ('prd', 'tech_spec'));

DROP TABLE action_items;
