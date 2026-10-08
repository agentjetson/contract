FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache git ca-certificates
COPY go.mod ./
RUN go mod download 2>/dev/null || true
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/object-storage ./cmd/server

FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
COPY --from=build /out/object-storage /usr/local/bin/object-storage
ENV DEMO_MODE=false
ENV HTTP_ADDR=0.0.0.0:8081
ENV GRPC_ADDR=0.0.0.0:50055
EXPOSE 8081 50055
ENTRYPOINT ["/usr/local/bin/object-storage"]
