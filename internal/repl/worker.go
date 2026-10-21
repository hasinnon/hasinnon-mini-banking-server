package repl

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"bank-ha/internal/config"
	"bank-ha/internal/model"
	"bank-ha/internal/security"
	"bank-ha/internal/store"
)

type LeaderChecker interface {
	IsLeader() bool
}

type Worker struct {
	cfg    config.Config
	st     *store.PrimaryStore
	leader LeaderChecker
	http   *http.Client
}

func NewWorker(cfg config.Config, st *store.PrimaryStore, leader LeaderChecker) *Worker {
	return &Worker{
		cfg:    cfg,
		st:     st,
		leader: leader,
		http: &http.Client{
			Timeout: time.Duration(cfg.HTTPTimeoutMS) * time.Millisecond,
		},
	}
}

func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Duration(w.cfg.PollIntervalMS) * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if !w.leader.IsLeader() {
				continue
			}
			w.tick()
		}
	}
}

func (w *Worker) tick() {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	events, err := w.st.PickEvents(ctx, w.cfg.BatchSize)
	cancel()
	if err != nil {
		log.Printf("replicator.pick error=%v", err)
		return
	}

	for _, ev := range events {
		if err := w.send(ev); err != nil {
			_ = w.st.MarkRetry(context.Background(), ev.Sequence, err)
			continue
		}
		_ = w.st.MarkAcked(context.Background(), ev.Sequence)
	}
}

func (w *Worker) send(ev model.ReplicateEvent) error {
	body, _ := json.Marshal(ev)
	req, _ := http.NewRequest(http.MethodPost, w.cfg.ReplicationURL, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", security.SignHMAC(body, w.cfg.ReplicationKey))

	resp, err := w.http.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var ack model.ReplicateAck
	_ = json.NewDecoder(resp.Body).Decode(&ack)

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusConflict:
		return fmt.Errorf("standby conflict: %s", ack.Message)
	default:
		return fmt.Errorf("standby status=%d", resp.StatusCode)
	}
}