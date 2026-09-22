CREATE TABLE notification_log (
    request_id TEXT        NOT NULL,
    outcome    TEXT        NOT NULL CHECK (outcome IN ('COMPLETED', 'FAILED')),
    sent_at    TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (request_id, outcome)
);
