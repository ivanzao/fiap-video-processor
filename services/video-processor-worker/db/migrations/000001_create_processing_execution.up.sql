CREATE TABLE processing_execution (
    request_id  TEXT PRIMARY KEY,
    attempt     INTEGER     NOT NULL CHECK (attempt >= 1),
    outcome     TEXT        NOT NULL CHECK (outcome IN ('RUNNING', 'RETRYING', 'COMPLETED', 'FAILED')),
    zip_key     TEXT,
    frame_count INTEGER CHECK (frame_count >= 0),
    reason      TEXT,
    started_at  TIMESTAMPTZ NOT NULL,
    finished_at TIMESTAMPTZ
);
