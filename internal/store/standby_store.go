package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"

	"bank-ha/internal/model"
)

type StandbyStore struct {
	db *sql.DB
}

func NewStandbyStore(db *sql.DB) *StandbyStore {
	return &StandbyStore{db: db}
}

func (s *StandbyStore) ApplyReplicatedEvent(ctx context.Context, ev model.ReplicateEvent) (model.ReplicateAck, int) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: err.Error()}, http.StatusInternalServerError
	}
	defer tx.Rollback()

	var x string
	err = tx.QueryRowContext(ctx, `SELECT event_id FROM applied_events WHERE event_id=$1`, ev.EventID).Scan(&x)
	if err == nil {
		_ = tx.Commit()
		return model.ReplicateAck{EventID: ev.EventID, Status: "duplicate"}, http.StatusOK
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: err.Error()}, http.StatusInternalServerError
	}

	var last int64
	if err := tx.QueryRowContext(ctx, `SELECT last_sequence FROM replication_cursor WHERE id=1 FOR UPDATE`).Scan(&last); err != nil {
		return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: err.Error()}, http.StatusInternalServerError
	}

	if ev.Sequence <= last {
		_ = tx.Commit()
		return model.ReplicateAck{EventID: ev.EventID, Status: "duplicate"}, http.StatusOK
	}
	if ev.Sequence != last+1 {
		return model.ReplicateAck{
			EventID: ev.EventID,
			Status:  "out_of_order",
			Message: fmt.Sprintf("expected=%d got=%d", last+1, ev.Sequence),
		}, http.StatusConflict
	}

	switch ev.EventType {
	case "deposit":
		res, err := tx.ExecContext(ctx, `
			UPDATE accounts
			SET balance = balance + $1, updated_at=now()
			WHERE account_id=$2
		`, ev.Amount, ev.AccountID)
		if err != nil {
			return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: err.Error()}, http.StatusInternalServerError
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: "unknown account"}, http.StatusConflict
		}

	case "withdraw":
		res, err := tx.ExecContext(ctx, `
			UPDATE accounts
			SET balance = balance - $1, updated_at=now()
			WHERE account_id=$2 AND balance >= $1
		`, ev.Amount, ev.AccountID)
		if err != nil {
			return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: err.Error()}, http.StatusInternalServerError
		}
		n, _ := res.RowsAffected()
		if n == 0 {
			return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: "insufficient_or_unknown"}, http.StatusConflict
		}
	default:
		return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: "unsupported event_type"}, http.StatusBadRequest
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO ledger(request_id,event_id,account_id,op,amount)
		VALUES ($1,$2,$3,$4,$5)
	`, ev.RequestID, ev.EventID, ev.AccountID, ev.EventType, ev.Amount); err != nil {
		return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: err.Error()}, http.StatusConflict
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO applied_events(event_id,sequence,request_id)
		VALUES ($1,$2,$3)
	`, ev.EventID, ev.Sequence, ev.RequestID); err != nil {
		return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: err.Error()}, http.StatusConflict
	}

	if _, err := tx.ExecContext(ctx, `UPDATE replication_cursor SET last_sequence=$1, updated_at=now() WHERE id=1`, ev.Sequence); err != nil {
		return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: err.Error()}, http.StatusInternalServerError
	}

	if err := tx.Commit(); err != nil {
		return model.ReplicateAck{EventID: ev.EventID, Status: "error", Message: err.Error()}, http.StatusInternalServerError
	}
	return model.ReplicateAck{EventID: ev.EventID, Status: "applied"}, http.StatusOK
}