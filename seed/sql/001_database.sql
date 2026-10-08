-- AgentJetson ClickHouse — database
-- Applied by docker-entrypoint-initdb.d and `make schema`.

CREATE DATABASE IF NOT EXISTS agentjetson;

-- Keep `default` usable for local compose that still points at CLICKHOUSE_DB=default.
-- Tables are created in the current database (CLICKHOUSE_DB / make schema ?database=).
