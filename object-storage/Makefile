.PHONY: run demo tidy build docker proto

run:
	DEMO_MODE=true go run ./cmd/server

demo: run

tidy:
	go mod tidy

build:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o bin/object-storage ./cmd/server

docker:
	docker compose up --build

# Requires buf CLI + protoc plugins.  Generates gen/ from proto/.
proto:
	@echo "Wire buf.gen.yaml when integrating with agentjetson/contract Buf workspace"
	@echo "  buf generate"
