CREATE TABLE artifacts (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id    uuid NOT NULL REFERENCES meetings (id) ON DELETE CASCADE,
    artifact_type text NOT NULL,
    content       text NOT NULL,
    created_at    timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT artifacts_type_check CHECK (artifact_type IN ('prd', 'tech_spec')),
    CONSTRAINT artifacts_meeting_type_unique UNIQUE (meeting_id, artifact_type)
);
