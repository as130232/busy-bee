CREATE TABLE meetings (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id           uuid NOT NULL REFERENCES users (id),
    title             text NOT NULL,
    audio_gcs_path    text NOT NULL DEFAULT '',
    status            text NOT NULL,
    transcript        text NOT NULL DEFAULT '',
    duration_seconds  int  NOT NULL DEFAULT 0,
    error_message     text NOT NULL DEFAULT '',
    scheduled_at      timestamptz,
    remind_before_min int  NOT NULL DEFAULT 15,
    processed_at      timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT meetings_status_check CHECK (
        status IN ('scheduled', 'pending', 'transcribing', 'analyzing', 'completed', 'failed')
    )
);

CREATE INDEX meetings_user_id_created_at_idx ON meetings (user_id, created_at DESC);
