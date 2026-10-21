package store

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"bank-ha/internal/model"

	"github.com/google/uuid"
)

type WitnessStore struct {
	db *sql.DB
}

func NewWitnessStore(db *sql.DB) *WitnessStore {
	return &WitnessStore{db: db}
}

// ClaimOrRenew:
// - اگر lease وجود ندارد => grant
// - اگر expired شده => grant به caller جدید (term++)
// - اگر holder خود caller است و token درست است => renew
// - غیر اینصورت deny
func (s *WitnessStore) ClaimOrRenew(ctx context.Context, req model.LeaseClaimRequest) (model.LeaseClaimResponse, error) {
	tx, err := s.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return model.LeaseClaimResponse{}, err
	}
	defer tx.Rollback()

	var holder string
	var token string
	var term int64
	var expiresAt time.Time

	err = tx.QueryRowContext(ctx, `
		SELECT holder_node_id, lease_token::text, term, expires_at
		FROM witness_leases
		WHERE cluster_name=$1
		FOR UPDATE
	`, req.Cluster).Scan(&holder, &token, &term, &expiresAt)

	now := time.Now().UTC()
	newExp := now.Add(time.Duration(req.TTLSeconds) * time.Second)

	// no row: create
	if errors.Is(err, sql.ErrNoRows) {
		newToken := uuid.NewString()
		term = 1
		_, err = tx.ExecContext(ctx, `
			INSERT INTO witness_leases(cluster_name, holder_node_id, lease_token, term, expires_at, updated_at)
			VALUES($1,$2,$3,$4,$5,now())
		`, req.Cluster, req.NodeID, newToken, term, newExp)
		if err != nil {
			return model.LeaseClaimResponse{}, err
		}
		if err := tx.Commit(); err != nil {
			return model.LeaseClaimResponse{}, err
		}
		return model.LeaseClaimResponse{
			Granted: true, Cluster: req.Cluster, Holder: req.NodeID, Token: newToken, Term: term, ExpiresAt: newExp,
		}, nil
	}
	if err != nil {
		return model.LeaseClaimResponse{}, err
	}

	// expired => takeover
	if now.After(expiresAt) {
		newToken := uuid.NewString()
		term = term + 1
		_, err = tx.ExecContext(ctx, `
			UPDATE witness_leases
			SET holder_node_id=$2, lease_token=$3, term=$4, expires_at=$5, updated_at=now()
			WHERE cluster_name=$1
		`, req.Cluster, req.NodeID, newToken, term, newExp)
		if err != nil {
			return model.LeaseClaimResponse{}, err
		}
		if err := tx.Commit(); err != nil {
			return model.LeaseClaimResponse{}, err
		}
		return model.LeaseClaimResponse{
			Granted: true, Cluster: req.Cluster, Holder: req.NodeID, Token: newToken, Term: term, ExpiresAt: newExp,
		}, nil
	}

	// renew by same holder + same token
	if holder == req.NodeID && req.CurrentToken != "" && req.CurrentToken == token {
		_, err = tx.ExecContext(ctx, `
			UPDATE witness_leases
			SET expires_at=$2, updated_at=now()
			WHERE cluster_name=$1
		`, req.Cluster, newExp)
		if err != nil {
			return model.LeaseClaimResponse{}, err
		}
		if err := tx.Commit(); err != nil {
			return model.LeaseClaimResponse{}, err
		}
		return model.LeaseClaimResponse{
			Granted: true, Cluster: req.Cluster, Holder: holder, Token: token, Term: term, ExpiresAt: newExp,
		}, nil
	}

	// deny
	if err := tx.Commit(); err != nil {
		return model.LeaseClaimResponse{}, err
	}
	return model.LeaseClaimResponse{
		Granted: false, Cluster: req.Cluster, Holder: holder, Token: token, Term: term, ExpiresAt: expiresAt,
		Message: "lease held by another node",
	}, nil
}