module github.com/agentjetson/voice-query-service

go 1.25.0

// Primary path is HTTP (used by C++ voice_agent via libcurl) + MCP.
// Protobuf contracts are generated via Buf into gen/.
//
// DEMO_MODE=true needs no external packages.
// For live ClickHouse:
//   go get github.com/ClickHouse/clickhouse-go/v2
// For optional gRPC surface / NATS live peek add:
//   google.golang.org/grpc
//   google.golang.org/protobuf
//   github.com/nats-io/nats.go

require (
	google.golang.org/grpc v1.84.0
	google.golang.org/protobuf v1.36.12
)

require (
	golang.org/x/net v0.57.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260706201446-f0a921348800 // indirect
)
