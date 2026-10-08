# syntax=docker/dockerfile:1.7
FROM golang:1.22-bookworm AS builder
WORKDIR /src

# Shared packages + generated stubs (monorepo context).
COPY pkg/natsjs /src/pkg/natsjs
COPY gen/go /src/gen/go
COPY nats-publisher /src/nats-publisher

WORKDIR /src/nats-publisher
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /out/nats-publisher ./cmd/publisher

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/nats-publisher /nats-publisher
USER nonroot:nonroot
ENTRYPOINT ["/nats-publisher"]
