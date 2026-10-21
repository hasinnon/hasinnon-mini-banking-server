package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"bank-ha/internal/model"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
)

type PrimaryStore struct {
	db              *sql.DB
	maxRetrySeconds int
}

func NewPrimaryStore(db *sql.DB, maxRetrySeconds int) *PrimaryStore {
	return &PrimaryStore{db: db, maxRetrySeconds: maxRetrySeconds}
}

func (s *PrimaryStore) GetBalance(ctx context.Context, accountID string) (int64, error) {
	var b int64
	err := s.db.QueryRowContext(ctx, `SELECT balance FROM accounts WHERE account_id=$1`, accountID).Scan(&b)
	if errors.Is(err, sql.ErrNoRows) {
		return 0, fmt.Errorf("unknown account")
	}
	return b, err
}

func (s *PrimaryStore) ApplyAndEnqueue(ctx context.Context, req model.TxRequest, op string) (int64, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	var bal int64
	if err := tx.QueryRowContext(ctx, `SELECT balance FROM accounts WHERE account_id=$1 FOR UPDATE`, req.AccountID).Scan(&bal); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return 0, fmt.Errorf("unknown account")
		}
		return 0, err
	}

	switch op {
	case "deposit":
		bal += req.Amount
	case "withdraw":
		if bal < req.Amount {
			return 0, fmt.Errorf("insufficient balance")
		}
		bal -= req.Amount
	default:
		return 0, fmt.Errorf("unsupported op")
	}

	if _, err := tx.ExecContext(ctx, `UPDATE accounts SET balance=$1, updated_at=now() WHERE account_id=$2`, bal, req.AccountID); err != nil {
		return 0, err
	}

	eventID := uuid.NewString()
	payload, _ := json.Marshal(map[string]any{
		"request_id": req.RequestID,
		"account_id": req.AccountID,
		"amount":     req.Amount,
		"op":         op,
	})

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ledger(request_id,event_id,account_id,op,amount)
		VALUES ($1,$2,$3,$4,$5)
	`, req.RequestID, eventID, req.AccountID, op, req.Amount); err != nil {
		if isUniqueViolation(err) {
			return 0, fmt.Errorf("duplicate request")
		}
		return 0, err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events(event_id,request_id,event_type,account_id,amount,payload,status)
		VALUES ($1,$2,$3,$4,$5,$6,'pending')
	`, eventID, req.RequestID, op, req.AccountID, req.Amount, payload); err != nil {
		return 0, err
	}

	return bal, tx.Commit()
}

func (s *PrimaryStore) PickEvents(ctx context.Context, limit int) ([]model.ReplicateEvent, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	rows, err := tx.QueryContext(ctx, `
	WITH cte AS (
	  SELECT id
	  FROM outbox_events
	  WHERE (status IN ('pending','retry') AND next_retry_at <= now())
	     OR (status='sending' AND sent_at < now() - interval '10 seconds')
	  ORDER BY id
	  LIMIT $1
	  FOR UPDATE SKIP LOCKED
	)
	UPDATE outbox_events o
	SET status='sending', sent_at=now(), tries=o.tries+1
	FROM cte
	WHERE o.id=cte.id
	RETURNING o.id,o.event_id,o.request_id,o.event_type,o.account_id,o.amount,o.payload,o.created_at
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := make([]model.ReplicateEvent, 0, limit)
	for rows.Next() {
		var e model.ReplicateEvent
		if err := rows.Scan(&e.Sequence, &e.EventID, &e.RequestID, &e.EventType, &e.AccountID, &e.Amount, &e.Payload, &e.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return out, nil
}

func (s *PrimaryStore) MarkAcked(ctx context.Context, seq int64) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status='acked', acked_at=now(), last_error=NULL
		WHERE id=$1
	`, seq)
	return err
}

func (s *PrimaryStore) MarkRetry(ctx context.Context, seq int64, cause error) error {
	var tries int
	if err := s.db.QueryRowContext(ctx, `SELECT tries FROM outbox_events WHERE id=$1`, seq).Scan(&tries); err != nil {
		return err
	}
	sec := int(math.Min(float64(s.maxRetrySeconds), math.Pow(2, float64(max(1, tries)))))
	_, err := s.db.ExecContext(ctx, `
		UPDATE outbox_events
		SET status='retry',
		    next_retry_at=now() + ($2 || ' seconds')::interval,
		    last_error=$3
		WHERE id=$1
	`, seq, sec, truncate(cause.Error(), 500))
	return err
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) {
		return pgErr.Code == "23505"
	}
	return strings.Contains(strings.ToLower(err.Error()), "duplicate key")
}
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}