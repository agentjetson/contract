module github.com/agentjetson/contract/ingest

go 1.22

require (
	github.com/agentjetson/contract/gen/go v0.0.0
	google.golang.org/grpc v1.67.1
	google.golang.org/protobuf v1.35.1
)

// Local monorepo replaces — adjust or drop when modules are published.
replace github.com/agentjetson/contract/gen/go => ../gen/go
