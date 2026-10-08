module github.com/agentjetson/core/clickhouse-consumer

go 1.22

require (
	github.com/agentjetson/core/gen/go v0.0.0
	github.com/agentjetson/core/pkg/persistence v0.0.0
	github.com/nats-io/nats.go v1.37.0
	google.golang.org/protobuf v1.35.1
)

replace github.com/agentjetson/core/pkg/persistence => ../pkg/persistence

replace github.com/agentjetson/core/gen/go => ../gen/go
