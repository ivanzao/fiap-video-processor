CREATE TABLE video_process_request (
    id         UUID PRIMARY KEY,
    video_id   UUID        NOT NULL UNIQUE REFERENCES video (id),
    user_id    TEXT        NOT NULL,
    status     TEXT        NOT NULL CHECK (status IN ('PENDING', 'PROCESSING', 'COMPLETED', 'FAILED')),
    attempts   INTEGER     NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE INDEX video_process_request_user_created_idx ON video_process_request (user_id, created_at DESC);
