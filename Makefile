# AgentJetson core
# Spins up ClickHouse. Schema lives here.
SHELL := /usr/bin/env bash
.SHELLFLAGS := -euo pipefail -c

COMPOSE     := docker compose -f docker-compose.yml
CH_USER     ?= default
CH_PASSWORD ?= pass
CH_DB       ?= default
CH_HTTP     ?= http://127.0.0.1:8123

CH_CLIENT = $(COMPOSE) exec -T clickhouse clickhouse-client \
	--multiquery \
	--database=$(CH_DB) \
	--user=$(CH_USER) \
	--password=$(CH_PASSWORD)

.PHONY: help up down reset logs ps ping schema seed client generate

help:
	@echo "AgentJetson core"
	@echo ""
	@echo "  make up         Start ClickHouse (8123 HTTP / 9000 native)"
	@echo "  make schema     Apply DDL (idempotent CREATE IF NOT EXISTS + views)"
	@echo "  make seed       Load demo rows (seed.cam-* / seed.mic-*)"
	@echo "  make ping       HTTP ping"
	@echo "  make logs       Tail ClickHouse logs"
	@echo "  make ps         Compose status"
	@echo "  make client     Interactive clickhouse-client"
	@echo "  make down       Stop containers"
	@echo "  make reset      Wipe volume and re-init (schema + seed)"
	@echo "  make generate   buf generate (Go stubs → gen/go)"
	@echo ""
	@echo "Consumers keep using:"
	@echo "  CLICKHOUSE_HOST=localhost CLICKHOUSE_PORT=9000"
	@echo "  CLICKHOUSE_USER=$(CH_USER) CLICKHOUSE_PASSWORD=$(CH_PASSWORD)"
	@echo "  CLICKHOUSE_DB=$(CH_DB)"

up:
	$(COMPOSE) up -d
	@echo "waiting for ClickHouse..."
	@for i in $$(seq 1 40); do \
		if curl -sf $(CH_HTTP)/ping >/dev/null; then \
			echo "ClickHouse ready on 8123 / 9000"; \
			exit 0; \
		fi; \
		sleep 0.4; \
	done; \
	echo "ClickHouse did not become ready"; exit 1

down:
	$(COMPOSE) down

reset:
	$(COMPOSE) down -v
	$(MAKE) up

logs:
	$(COMPOSE) logs -f clickhouse

ps:
	$(COMPOSE) ps

ping:
	curl -sf $(CH_HTTP)/ping && echo

schema: up
	@echo "==> 002_tables.sql"
	@$(CH_CLIENT) < seed/sql/002_tables.sql
	@echo "==> 003_views.sql"
	@$(CH_CLIENT) < seed/sql/003_views.sql
	@echo "schema applied"

seed: schema
	@echo "==> 004_seed.sql"
	@$(CH_CLIENT) < seed/sql/004_seed.sql
	@echo "seed applied"

client:
	$(COMPOSE) exec clickhouse clickhouse-client \
		--database=$(CH_DB) \
		--user=$(CH_USER) \
		--password=$(CH_PASSWORD)

generate:
	@command -v buf >/dev/null || (echo "install buf: https://buf.build/docs/installation" && exit 1)
	buf generate
