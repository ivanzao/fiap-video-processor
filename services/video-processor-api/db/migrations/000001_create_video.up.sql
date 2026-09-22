CREATE TABLE video (
    id           UUID PRIMARY KEY,
    user_id      TEXT        NOT NULL,
    filename     TEXT        NOT NULL,
    content_type TEXT        NOT NULL,
    size_bytes   BIGINT      NOT NULL CHECK (size_bytes >= 0),
    object_key   TEXT        NOT NULL UNIQUE,
    created_at   TIMESTAMPTZ NOT NULL
);

CREATE INDEX video_user_id_idx ON video (user_id);
