CREATE TABLE IF NOT EXISTS replication_cursor (
  id SMALLINT PRIMARY KEY DEFAULT 1,
  last_sequence BIGINT NOT NULL DEFAULT 0,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO replication_cursor(id, last_sequence)
VALUES (1, 0)
ON CONFLICT (id) DO NOTHING;

CREATE TABLE IF NOT EXISTS applied_events (
  event_id UUID PRIMARY KEY,
  sequence BIGINT NOT NULL UNIQUE,
  request_id UUID NOT NULL UNIQUE,
  applied_at TIMESTAMPTZ NOT NULL DEFAULT now()
);