# syntax=docker/dockerfile:1.7
FROM golang:1.26-bookworm AS builder
WORKDIR /src

# Shared packages + generated stubs (monorepo context).
COPY gen/go /src/gen/go
COPY ingest /src/ingest

WORKDIR /src/ingest
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /out/ingest-server ./cmd/server

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/ingest-server /ingest-server
USER nonroot:nonroot
ENTRYPOINT ["/ingest-server"]
