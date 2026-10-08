module github.com/agentjetson/contract/clickhouse-consumer

go 1.22

require (
	github.com/agentjetson/contract/gen/go v0.0.0
	github.com/agentjetson/contract/pkg/persistence v0.0.0
	github.com/nats-io/nats.go v1.37.0
	google.golang.org/protobuf v1.35.1
)

replace github.com/agentjetson/contract/pkg/persistence => ../pkg/persistence

replace github.com/agentjetson/contract/gen/go => ../gen/go
