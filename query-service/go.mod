module github.com/agentjetson/voice-query-service

go 1.22

// HTTP (C++ voice_agent via libcurl) + MCP JSON-line.
// Live ClickHouse via pkg/persistence (Conn + SELECTs on query_* views).
// Do NOT call ApplySchema — schema is owned by contract/seed/sql (make schema).

require (
	github.com/agentjetson/contract/pkg/persistence v0.0.0
)

replace github.com/agentjetson/contract/pkg/persistence => ../pkg/persistence
