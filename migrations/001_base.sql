CREATE TABLE IF NOT EXISTS accounts (
  account_id TEXT PRIMARY KEY,
  balance BIGINT NOT NULL CHECK (balance >= 0),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS ledger (
  id BIGSERIAL PRIMARY KEY,
  request_id UUID NOT NULL UNIQUE,
  event_id UUID NOT NULL UNIQUE,
  account_id TEXT NOT NULL,
  op TEXT NOT NULL CHECK (op IN ('deposit', 'withdraw')),
  amount BIGINT NOT NULL CHECK (amount > 0),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

INSERT INTO accounts(account_id, balance)
VALUES ('A1001', 100000), ('A1002', 50000), ('A1003', 75000)
ON CONFLICT (account_id) DO NOTHING;