package role

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"sync"
	"time"

	"bank-ha/internal/config"
	"bank-ha/internal/model"
	"bank-ha/internal/security"
)

type Manager struct {
	cfg  config.Config
	http *http.Client

	mu       sync.RWMutex
	isLeader bool
	token    string
	term     int64
	expires  time.Time
}

func NewManager(cfg config.Config) *Manager {
	return &Manager{
		cfg: cfg,
		http: &http.Client{
			Timeout: time.Duration(cfg.HTTPTimeoutMS) * time.Millisecond,
		},
	}
}

func (m *Manager) Run(ctx context.Context) {
	tk := time.NewTicker(time.Duration(m.cfg.RenewIntervalMS) * time.Millisecond)
	defer tk.Stop()

	m.tick() // immediately
	for {
		select {
		case <-ctx.Done():
			return
		case <-tk.C:
			m.tick()
		}
	}
}

func (m *Manager) IsLeader() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isLeader && time.Now().UTC().Before(m.expires)
}

func (m *Manager) tick() {
	m.mu.RLock()
	curToken := m.token
	m.mu.RUnlock()

	reqObj := model.LeaseClaimRequest{
		Cluster:      m.cfg.ClusterName,
		NodeID:       m.cfg.NodeID,
		TTLSeconds:   m.cfg.LeaseTTLSeconds,
		CurrentToken: curToken,
	}
	body, _ := json.Marshal(reqObj)

	req, _ := http.NewRequest(http.MethodPost, m.cfg.WitnessURL+"/v1/lease/claim", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Signature", security.SignHMAC(body, m.cfg.WitnessKey))

	resp, err := m.http.Do(req)
	if err != nil {
		m.demote("witness unreachable")
		return
	}
	defer resp.Body.Close()

	var out model.LeaseClaimResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		m.demote("bad witness response")
		return
	}

	if resp.StatusCode != http.StatusOK || !out.Granted || out.Holder != m.cfg.NodeID {
		m.demote("lease not granted")
		return
	}

	m.mu.Lock()
	wasLeader := m.isLeader
	m.isLeader = true
	m.token = out.Token
	m.term = out.Term
	m.expires = out.ExpiresAt
	m.mu.Unlock()

	if !wasLeader {
		log.Printf("ROLE CHANGE -> LEADER term=%d exp=%s", out.Term, out.ExpiresAt.Format(time.RFC3339))
	}
}

func (m *Manager) demote(reason string) {
	m.mu.Lock()
	wasLeader := m.isLeader
	m.isLeader = false
	m.mu.Unlock()

	if wasLeader {
		log.Printf("ROLE CHANGE -> FOLLOWER reason=%s", reason)
	}
}