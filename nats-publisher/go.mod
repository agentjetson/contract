module github.com/agentjetson/core/nats-publisher

go 1.26

require (
	github.com/agentjetson/core/pkg/natsjs v0.0.0
	github.com/nats-io/nats.go v1.37.0
	google.golang.org/grpc v1.67.1
	google.golang.org/protobuf v1.35.1
)

// Local monorepo replaces — adjust or drop when modules are published.
replace github.com/agentjetson/core/pkg/natsjs => ../pkg/natsjs

// Generated stubs (run `make generate` at contract root).
// Uncomment / adjust once gen/go is present:
// require github.com/agentjetson/core/gen/go v0.0.0
// replace github.com/agentjetson/core/gen/go => ../gen/go
