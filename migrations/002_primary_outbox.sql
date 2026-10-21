CREATE TABLE IF NOT EXISTS outbox_events (
  id BIGSERIAL PRIMARY KEY,
  event_id UUID NOT NULL UNIQUE,
  request_id UUID NOT NULL UNIQUE,
  event_type TEXT NOT NULL CHECK (event_type IN ('deposit', 'withdraw')),
  account_id TEXT NOT NULL,
  amount BIGINT NOT NULL CHECK (amount > 0),
  payload JSONB NOT NULL,
  status TEXT NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','sending','retry','acked')),
  tries INT NOT NULL DEFAULT 0,
  next_retry_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_error TEXT,
  sent_at TIMESTAMPTZ,
  acked_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_outbox_dispatch
  ON outbox_events(status, next_retry_at, id);