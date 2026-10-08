# syntax=docker/dockerfile:1
#
# Shared multi-stage Dockerfile aligned with balanz-nats-poc.
# Generates protobuf/gRPC stubs inside the image so `docker compose build`
# works from a clean checkout even though generated *.pb.go files are gitignored.

# ---------------------------------------------------------------------------
# Stage 1: generate protobuf stubs with Buf
# ---------------------------------------------------------------------------
FROM golang:1.22-alpine AS proto
RUN apk add --no-cache curl && \
    curl -sSL "https://github.com/bufbuild/buf/releases/latest/download/buf-Linux-x86_64" \
      -o /usr/local/bin/buf && chmod +x /usr/local/bin/buf
WORKDIR /src
COPY go.mod go.sum* ./
RUN go install google.golang.org/protobuf/cmd/protoc-gen-go@latest && \
    go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
ENV PATH="/root/go/bin:${PATH}"
COPY buf.gen.yaml buf.yaml* ./
COPY proto ./proto
RUN buf generate

# ---------------------------------------------------------------------------
# Stage 2: build the service binary
# ---------------------------------------------------------------------------
FROM --platform=$BUILDPLATFORM golang:1.22-alpine AS build
ARG SERVICE=server
ARG TARGETOS
ARG TARGETARCH
WORKDIR /src
COPY go.mod go.sum* ./
RUN go mod download 2>/dev/null || true
COPY --from=proto /src .
COPY . .

RUN CGO_ENABLED=0 \
    GOOS=${TARGETOS:-linux} \
    GOARCH=${TARGETARCH:-amd64} \
    go build -ldflags="-s -w" -o /out/service ./cmd/${SERVICE}

# ---------------------------------------------------------------------------
# Stage 3: minimal runtime
# ---------------------------------------------------------------------------
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
COPY --from=build /out/service /app/voice-query-service
EXPOSE 8080
ENV HTTP_ADDR=:8080
ENTRYPOINT ["/app/voice-query-service"]
