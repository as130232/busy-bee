ALTER TABLE meetings
    DROP COLUMN IF EXISTS transcript_segments,
    DROP COLUMN IF EXISTS speaker_names;
