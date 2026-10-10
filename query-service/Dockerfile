# syntax=docker/dockerfile:1.7
# Build from monorepo root:
#   docker build -f voice-query-service/Dockerfile -t voice-query-service .
#
# Uses pkg/persistence for ClickHouse; schema is owned by seed/sql.

FROM golang:1.26-bookworm AS builder
WORKDIR /src

COPY pkg/persistence /src/pkg/persistence
COPY pkg/otel /src/pkg/otel
COPY voice-query-service /src/voice-query-service

WORKDIR /src/voice-query-service
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/voice-query-service ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/voice-query-service /voice-query-service
USER nonroot:nonroot
EXPOSE 8080
ENV HTTP_ADDR=:8080
ENTRYPOINT ["/voice-query-service"]
