package main

import (
	"context"
	"log"
	"net/http"
	"time"

	"bank-ha/internal/config"
	"bank-ha/internal/db"
	"bank-ha/internal/httpapi"
	"bank-ha/internal/repl"
	"bank-ha/internal/role"
	"bank-ha/internal/store"
)

func main() {
	cfg := config.MustLoad()

	database := db.MustOpen(cfg.DatabaseURL)
	defer database.Close()

	mux := http.NewServeMux()
	mux.HandleFunc("/health", httpapi.Health(cfg.Mode))

	switch cfg.Mode {
	case "witness":
		ws := store.NewWitnessStore(database)
		wh := httpapi.NewWitnessHandler(cfg, ws)
		mux.HandleFunc("POST /v1/lease/claim", wh.ClaimLease)
		log.Println("mode=witness")

	case "node":
		primaryStore := store.NewPrimaryStore(database, cfg.MaxRetrySeconds)
		standbyStore := store.NewStandbyStore(database)

		rm := role.NewManager(cfg)
		go rm.Run(context.Background())

		ph := httpapi.NewPrimaryHandler(primaryStore, rm)
		sh := httpapi.NewStandbyHandler(cfg, standbyStore, rm)

		mux.HandleFunc("GET /api/v1/balance", ph.Balance)
		mux.HandleFunc("POST /api/v1/deposit", ph.Deposit)
		mux.HandleFunc("POST /api/v1/withdraw", ph.Withdraw)
		mux.HandleFunc("POST /internal/replicate/event", sh.ApplyReplicatedEvent)

		worker := repl.NewWorker(cfg, primaryStore, rm)
		go worker.Run(context.Background())

		log.Println("mode=node")
	default:
		log.Fatalf("invalid MODE=%q (node|witness)", cfg.Mode)
	}

	handler := httpapi.Chain(
		mux,
		httpapi.MaxConcurrent(cfg.MaxConcurrentClients),
		httpapi.RequestID(),
		httpapi.AccessLog(),
	)

	srv := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}
	log.Printf("listen=%s", cfg.ListenAddr)
	log.Fatal(srv.ListenAndServe())
}