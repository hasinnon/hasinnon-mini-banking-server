#!/usr/bin/env bash
set -euo pipefail
set -a
source .env.standby.example
set +a
go run ./cmd/server