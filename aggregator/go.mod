module github.com/agentjetson/core/aggregator

go 1.26

require (
	github.com/agentjetson/core/gen/go v0.0.0
	github.com/agentjetson/core/pkg/otel v0.0.0
	github.com/nats-io/nats.go v1.37.0
	google.golang.org/protobuf v1.36.1
)

require (
	github.com/klauspost/compress v1.17.2 // indirect
	github.com/nats-io/nkeys v0.4.7 // indirect
	github.com/nats-io/nuid v1.0.1 // indirect
	golang.org/x/crypto v0.27.0 // indirect
	golang.org/x/net v0.29.0 // indirect
	golang.org/x/sys v0.25.0 // indirect
	golang.org/x/text v0.18.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20240903143218-8af14fe29dc1 // indirect
	google.golang.org/grpc v1.68.1 // indirect
)

replace github.com/agentjetson/core/gen/go => ../gen/go

replace github.com/agentjetson/core/pkg/otel => ../pkg/otel
