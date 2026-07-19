CREATE TABLE action_items (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id  uuid NOT NULL REFERENCES meetings (id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    description text NOT NULL,
    assignee    text NOT NULL DEFAULT '',
    due_text    text NOT NULL DEFAULT '',
    done        boolean NOT NULL DEFAULT false,
    sort_order  int NOT NULL DEFAULT 0,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX action_items_user_pending_idx ON action_items (user_id, done, created_at);
CREATE INDEX action_items_meeting_idx ON action_items (meeting_id);

-- artifacts 允許 action_items 類型（抽取階段以原始 JSON 作為冪等標記）
ALTER TABLE artifacts DROP CONSTRAINT artifacts_type_check;
ALTER TABLE artifacts ADD CONSTRAINT artifacts_type_check
    CHECK (artifact_type IN ('prd', 'tech_spec', 'action_items'));
