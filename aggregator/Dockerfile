# syntax=docker/dockerfile:1.7
# Build from monorepo root:
#   docker build -f aggregator/Dockerfile -t aggregator .
FROM golang:1.26-bookworm AS builder
WORKDIR /src

RUN curl -sSL "https://github.com/bufbuild/buf/releases/download/v1.47.2/buf-Linux-x86_64" \
    -o /usr/local/bin/buf && chmod +x /usr/local/bin/buf

COPY buf.yaml buf.gen.yaml /src/
COPY proto /src/proto
COPY pkg/otel /src/pkg/otel
COPY aggregator /src/aggregator

# gen/go is not committed — generate at build time
RUN buf generate
RUN printf 'module github.com/agentjetson/core/gen/go\n\ngo 1.26\n\nrequire google.golang.org/protobuf v1.36.1\n' \
    > /src/gen/go/go.mod

WORKDIR /src/aggregator
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/aggregator ./cmd/aggregator

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/aggregator /aggregator
USER nonroot:nonroot
ENTRYPOINT ["/aggregator"]
