# Makefile for voice-query-service (aligned with balanz-nats-poc Buf workflow)
SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

export PATH := /tmp:$(PATH)

.PHONY: all setup generate tidy clean up down build logs ps run run-demo info help

all: setup

## setup: generate stubs, tidy modules, then print how to run
setup: generate tidy info

## generate: run buf generate to produce protobuf / gRPC stubs
generate:
	@echo "==> Generating protobuf / gRPC stubs with Buf"
	buf generate

## tidy: tidy go.mod / go.sum
tidy:
	@echo "==> Tidying Go modules"
	go mod tidy

## clean: remove generated protobuf / gRPC stubs
clean:
	@echo "==> Removing generated protobuf / gRPC stubs"
	@find . -type f \( -name '*.pb.go' -o -name '*_grpc.pb.go' -o -name '*.pb.gw.go' \) \
		-not -path './vendor/*' -print -delete
	@find . -type d -name 'gen' -not -path './vendor/*' -prune -exec rm -rf {} \; -print || true
	@mkdir -p gen

## up: build (if needed) and start the service (and optional deps)
up:
	docker compose up -d --build

## down: stop and remove containers
down:
	docker compose down -v

## build: (re)build service image
build:
	docker compose build

## logs: tail logs
logs:
	docker compose logs -f

## ps: show status
ps:
	docker compose ps

## run: start the HTTP + MCP server locally
run:
	go run ./cmd/server

## run-demo: start in DEMO_MODE (no ClickHouse required)
run-demo:
	DEMO_MODE=true go run ./cmd/server

## info: print instructions
info:
	@echo ""
	@echo "============================================================"
	@echo "  voice-query-service is ready (Buf-aligned)."
	@echo "============================================================"
	@echo ""
	@echo "  Generate + tidy:"
	@echo "       make generate && make tidy"
	@echo ""
	@echo "  Local (demo, no ClickHouse):"
	@echo "       make run-demo"
	@echo "       curl -s http://localhost:8080/health | jq"
	@echo ""
	@echo "  Against real ClickHouse (same env as AgentJetson core):"
	@echo "       export CLICKHOUSE_HOST=localhost"
	@echo "       export CLICKHOUSE_PORT=9000"
	@echo "       export CLICKHOUSE_USER=default"
	@echo "       export CLICKHOUSE_PASSWORD=pass"
	@echo "       export APPLY_SCHEMA=true"
	@echo "       export DEMO_MODE=false"
	@echo "       make run"
	@echo ""
	@echo "  Docker:"
	@echo "       make up            # build + start"
	@echo "       make logs"
	@echo "       make down"
	@echo ""
	@echo "============================================================"

## help: list available targets
help:
	@grep -E '^## ' Makefile | sed -e 's/## /  /'
