module github.com/agentjetson/core/ingest

go 1.26

require (
	github.com/agentjetson/core/gen/go v0.0.0
	google.golang.org/grpc v1.67.1
)

require (
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
	google.golang.org/protobuf v1.35.1 // indirect
)

// Local monorepo replaces — adjust or drop when modules are published.
replace github.com/agentjetson/core/gen/go => ../gen/go
