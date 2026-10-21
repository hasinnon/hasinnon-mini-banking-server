package config

import (
	"log"
	"os"
	"strconv"
	"strings"
)

type Config struct {
	Mode        string // node | witness
	DatabaseURL string
	ListenAddr  string

	// replication
	ReplicationURL  string
	ReplicationKey  string
	PollIntervalMS  int
	BatchSize       int
	HTTPTimeoutMS   int
	MaxRetrySeconds int

	// throttling
	MaxConcurrentClients int

	// lease/witness
	ClusterName     string
	NodeID          string
	WitnessURL      string
	WitnessKey      string
	LeaseTTLSeconds int
	RenewIntervalMS int
}

func MustLoad() Config {
	cfg := Config{
		Mode:                 get("MODE", "node"),
		DatabaseURL:          must("DATABASE_URL"),
		ListenAddr:           get("LISTEN_ADDR", ":8080"),
		ReplicationURL:       get("REPLICATION_URL", "http://127.0.0.1:8081/internal/replicate/event"),
		ReplicationKey:       get("REPLICATION_KEY", "change-me"),
		PollIntervalMS:       atoi(get("POLL_INTERVAL_MS", "250"), 250),
		BatchSize:            atoi(get("BATCH_SIZE", "32"), 32),
		HTTPTimeoutMS:        atoi(get("HTTP_TIMEOUT_MS", "1500"), 1500),
		MaxRetrySeconds:      atoi(get("MAX_RETRY_SECONDS", "30"), 30),
		MaxConcurrentClients: atoi(get("MAX_CONCURRENT_CLIENTS", "200"), 200),

		ClusterName:     get("CLUSTER_NAME", "bank-cluster-1"),
		NodeID:          get("NODE_ID", ""),
		WitnessURL:      get("WITNESS_URL", "http://127.0.0.1:8090"),
		WitnessKey:      get("WITNESS_KEY", "witness-secret"),
		LeaseTTLSeconds: atoi(get("LEASE_TTL_SECONDS", "6"), 6),
		RenewIntervalMS: atoi(get("RENEW_INTERVAL_MS", "1500"), 1500),
	}

	if cfg.Mode == "node" && strings.TrimSpace(cfg.NodeID) == "" {
		log.Fatalf("NODE_ID is required when MODE=node")
	}
	return cfg
}

func get(k, d string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		return d
	}
	return v
}
func must(k string) string {
	v := strings.TrimSpace(os.Getenv(k))
	if v == "" {
		log.Fatalf("%s is required", k)
	}
	return v
}
func atoi(s string, d int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return v
}