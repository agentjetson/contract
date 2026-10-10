# pkg/otel

Shared OpenTelemetry bootstrap for AgentJetson Go microservices.

Traces are exported over OTLP/gRPC to the `otel-collector` service already
present in `docker-compose.yml`.

## Usage

```go
ctx := context.Background()
shutdown, err := otel.Init(ctx, "ingest")
if err != nil {
    slog.Error("otel", "err", err)
    os.Exit(1)
}
defer func() { _ = shutdown(context.Background()) }()

// gRPC server
grpcServer := grpc.NewServer(otel.GRPCServerOption())

// gRPC client
conn, err := grpc.NewClient(addr, append(otel.GRPCDialOptions(), grpc.WithTransportCredentials(...))...)

// HTTP
http.ListenAndServe(addr, otel.HTTPHandler("http.server", mux))
```

## Environment

| Variable | Default | Meaning |
|----------|---------|---------|
| `OTEL_SERVICE_NAME` | argument to `Init` | Resource `service.name` |
| `OTEL_EXPORTER_OTLP_ENDPOINT` | `otel-collector:4317` | Collector host:port |
| `OTEL_EXPORTER_OTLP_INSECURE` | `true` | Skip TLS (local stack) |
| `OTEL_SDK_DISABLED` | unset | Set `true` to no-op |

## Consumers

All Go services in this monorepo call `otel.Init` at startup and pass
`otel.GRPCServerOption` / `otel.HTTPHandler` where applicable.
