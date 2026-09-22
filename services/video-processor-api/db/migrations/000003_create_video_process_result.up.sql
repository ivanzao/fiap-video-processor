CREATE TABLE video_process_result (
    request_id     UUID PRIMARY KEY REFERENCES video_process_request (id),
    zip_key        TEXT,
    frame_count    INTEGER CHECK (frame_count >= 0),
    failure_reason TEXT,
    recorded_at    TIMESTAMPTZ NOT NULL,
    CHECK (zip_key IS NOT NULL OR failure_reason IS NOT NULL)
);
