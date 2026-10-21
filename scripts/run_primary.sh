#!/usr/bin/env bash
set -euo pipefail
set -a
source .env.primary.example
set +a
go run ./cmd/server