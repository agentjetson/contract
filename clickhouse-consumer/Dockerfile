# syntax=docker/dockerfile:1.7
FROM golang:1.22-bookworm AS builder
WORKDIR /src

# Install buf for protobuf generation
RUN curl -sSL "https://github.com/bufbuild/buf/releases/download/v1.47.2/buf-Linux-x86_64" \
    -o /usr/local/bin/buf && chmod +x /usr/local/bin/buf

# Proto sources + buf config + shared modules
COPY buf.yaml buf.gen.yaml /src/
COPY proto /src/proto
COPY pkg/persistence /src/pkg/persistence
COPY clickhouse-consumer /src/clickhouse-consumer

# Generate Go stubs into gen/go (matches go.mod replace)
RUN buf generate
RUN printf 'module github.com/agentjetson/contract/gen/go\n\ngo 1.22\n\nrequire google.golang.org/protobuf v1.35.1\n' \
    > /src/gen/go/go.mod

WORKDIR /src/clickhouse-consumer
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /out/clickhouse-consumer ./cmd/consumer

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/clickhouse-consumer /clickhouse-consumer
USER nonroot:nonroot
ENTRYPOINT ["/clickhouse-consumer"]
