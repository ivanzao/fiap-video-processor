CREATE TABLE outbox (
    id           BIGSERIAL PRIMARY KEY,
    event_id     UUID        NOT NULL UNIQUE,
    event_type   TEXT        NOT NULL,
    event_version INTEGER    NOT NULL DEFAULT 1,
    payload      JSONB       NOT NULL,
    occurred_at  TIMESTAMPTZ NOT NULL,
    published_at TIMESTAMPTZ
);

CREATE INDEX outbox_pending_idx ON outbox (id) WHERE published_at IS NULL;
