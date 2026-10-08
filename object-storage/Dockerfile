# Build from monorepo root:
#   docker build -f object-storage/Dockerfile -t object-storage .
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates

# Shared persistence module first (replace path in go.mod points here).
COPY pkg/persistence ./pkg/persistence

COPY object-storage/go.mod object-storage/go.sum ./object-storage/
WORKDIR /src/object-storage
RUN go mod download

COPY object-storage/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/object-storage ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/object-storage /usr/local/bin/object-storage
ENV DEMO_MODE=false
ENV HTTP_ADDR=0.0.0.0:8081
ENV GRPC_ADDR=0.0.0.0:50055
EXPOSE 8081 50055
ENTRYPOINT ["/usr/local/bin/object-storage"]
