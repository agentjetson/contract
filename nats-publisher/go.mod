module github.com/agentjetson/core/nats-publisher

go 1.26

require (
	github.com/agentjetson/core/gen/go v0.0.0
	github.com/agentjetson/core/pkg/natsjs v0.0.0
	github.com/agentjetson/core/pkg/otel v0.0.0
	github.com/nats-io/nats.go v1.37.0
	google.golang.org/grpc v1.67.1
	google.golang.org/protobuf v1.35.1
)

require (
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.26.0 // indirect
	golang.org/x/net v0.28.0 // indirect
	golang.org/x/sys v0.24.0 // indirect
	golang.org/x/text v0.17.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240814211410-ddb44dafa142 // indirect
)

// Local monorepo replaces — adjust or drop when modules are published.
replace github.com/agentjetson/core/gen/go => ../gen/go

replace github.com/agentjetson/core/pkg/natsjs => ../pkg/natsjs

replace github.com/agentjetson/core/pkg/otel => ../pkg/otel
