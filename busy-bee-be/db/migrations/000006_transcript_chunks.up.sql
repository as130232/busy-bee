CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE transcript_chunks (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    meeting_id  uuid NOT NULL REFERENCES meetings (id) ON DELETE CASCADE,
    user_id     uuid NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    chunk_index int  NOT NULL,
    content     text NOT NULL,
    embedding   vector(768) NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX transcript_chunks_embedding_idx ON transcript_chunks USING hnsw (embedding vector_cosine_ops);
CREATE INDEX transcript_chunks_meeting_idx ON transcript_chunks (meeting_id);
