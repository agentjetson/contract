# syntax=docker/dockerfile:1.7
FROM golang:1.26-bookworm AS builder
WORKDIR /src

COPY gen/go /src/gen/go
COPY pkg/otel /src/pkg/otel
COPY domain /src/domain
COPY scene-gate /src/scene-gate

WORKDIR /src/scene-gate
RUN go mod tidy && CGO_ENABLED=0 GOOS=linux go build -o /out/scene-gate ./cmd/scene-gate

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=builder /out/scene-gate /scene-gate
COPY --from=builder /src/domain/taxonomy.yaml /config/taxonomy.yaml
COPY --from=builder /src/domain/camera_profiles.yaml /config/camera_profiles.yaml
COPY --from=builder /src/domain/camera_sources.yaml /config/camera_sources.yaml
USER nonroot:nonroot
ENTRYPOINT ["/scene-gate"]
