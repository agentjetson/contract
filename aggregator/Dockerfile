# syntax=docker/dockerfile:1.7
FROM golang:1.22-bookworm AS builder
WORKDIR /src

COPY gen/go /src/gen/go
COPY aggregator /src/aggregator

WORKDIR /src/aggregator
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /out/aggregator ./cmd/aggregator

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/aggregator /aggregator
USER nonroot:nonroot
ENTRYPOINT ["/aggregator"]
