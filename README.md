```json
{
  "request_id": "b0f84e14-7cf5-4f12-b8fb-7b5d26cb57df",
  "account_id": "A1001",
  "amount": 1000
}

# Distributed Banking System

A small client–server banking system written in **Go**, using **PostgreSQL** for persistent storage. The project demonstrates concurrent transaction processing, application-level replication, automatic failover, and network-partition handling.

## Features

- Balance inquiry, deposits, and withdrawals.
- Transactional updates and concurrency control.
- Validation of malformed requests, invalid amounts, unknown accounts, and insufficient funds.
- Request and response logging, including request ID, client IP, operation, account ID, timestamp, result, and latency.
- Configurable maximum number of concurrent clients.
- Primary–Standby deployment with a Witness and leader leases.

## Architecture

```text
                 Client
                   |
          +--------+--------+
          |                 |
       Primary ---------- Standby
          |   Replication   |
          +--------+--------+
                   |
                 Witness
```

Each banking node uses its own PostgreSQL database. Data replication is implemented in Go rather than through PostgreSQL's built-in replication.

The Witness coordinates leadership through leases. A node must hold a valid lease before accepting writes. Losing contact with the Primary alone must not allow the Standby to become a leader.

**Important:** Leader leases prevent conflicting leadership only when correctly enforced. Safe failover also requires checking the Standby's replication state; leases alone do not guarantee preservation of every acknowledged transaction.

## Requirements

- Go, using the version specified in `go.mod`
- PostgreSQL
- Ubuntu Server virtual machines or equivalent Linux hosts
- Network connectivity between the client, banking nodes, and Witness

Go binaries can also be cross-compiled on Windows for deployment to Linux.

## Getting Started

1. Install PostgreSQL on both banking nodes.
2. Create a separate database and database user for each node.
3. Apply the project's database schema.
4. Configure node addresses, database connections, peer addresses, Witness address, and lease settings.
5. Build the project:

   ```bash
   go mod download
   go build ./...
   ```

6. Start the Witness and both banking nodes using the project's executable commands and configuration files.
7. Run the client on a separate machine.

## Example Request

The following example assumes the deposit endpoint is `/api/v1/deposit`:

```bash
curl -i -X POST http://192.168.56.11:8080/api/v1/deposit \
  -H "Content-Type: application/json" \
  -d '{
    "request_id": "demo-deposit-001",
    "account_id": "A1001",
    "amount": 1000
  }'
```

The account must exist before submitting the request. Use a new request ID for each new operation. Safe retries with the same ID require implemented idempotency support.


## APIs (Primary)
- `GET /api/v1/balance?account_id=A1001`
- `POST /api/v1/deposit`
- `POST /api/v1/withdraw`

## Test Scenarios

- Successful balance inquiries, deposits, and withdrawals.
- Invalid requests and insufficient funds.
- Concurrent deposits and withdrawals.
- Replication from Primary to Standby.
- Primary failure and safe Standby promotion.
- Recovery and synchronization of a restarted node.
- Network partition between Primary and Standby while both can reach the Witness.

During a network partition, **both nodes must never commit independent writes as leaders**. Depending on the replication policy, writes may be rejected or temporarily unavailable to preserve consistency.
