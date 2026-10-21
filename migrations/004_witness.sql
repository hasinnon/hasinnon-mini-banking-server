CREATE TABLE IF NOT EXISTS witness_leases (
  cluster_name TEXT PRIMARY KEY,
  holder_node_id TEXT NOT NULL,
  lease_token UUID NOT NULL,
  term BIGINT NOT NULL DEFAULT 1,
  expires_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_witness_leases_expires ON witness_leases(expires_at);