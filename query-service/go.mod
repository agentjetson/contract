module github.com/agentjetson/voice-query-service

go 1.22

// Primary path is HTTP (used by C++ voice_agent via libcurl) + MCP.
// Protobuf contracts are generated via Buf into gen/ (or from parent proto/).
//
// DEMO_MODE=true needs no external packages.
// For live ClickHouse prefer the shared client:
//   github.com/agentjetson/contract/pkg/persistence  (Conn() + SELECTs on query_* views)
// Do NOT call ApplySchema — schema is owned by contract/seed/sql (make schema).

require (
	github.com/agentjetson/contract/pkg/persistence v0.0.0
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

replace github.com/agentjetson/contract/pkg/persistence => ../pkg/persistence

require (
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
)
