# syntax=docker/dockerfile:1.7
FROM golang:1.22-bookworm AS builder
WORKDIR /src

# Shared persistence module (sibling path in monorepo context).
COPY pkg/persistence /src/pkg/persistence
COPY clickhouse-consumer /src/clickhouse-consumer

WORKDIR /src/clickhouse-consumer
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /out/clickhouse-consumer ./cmd/consumer

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/clickhouse-consumer /clickhouse-consumer
USER nonroot:nonroot
ENTRYPOINT ["/clickhouse-consumer"]
