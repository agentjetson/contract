<!-- Part of the AgentJetson Voice monorepo: https://github.com/agentjetson/voice -->
<!-- Schema: sql/002_envelopes.sql. DEMO_MODE=true for local work without ClickHouse. -->
<!-- See root README.md for architecture, bring-up order, and P0–P3 roadmap. -->

# Voice Query Service

## Design Principles

- Specialists (and this query service) are pure consumers of stable contracts.
- ObjectEnvelope / SceneResult / CapabilityResult stay the source of truth.
- Edge does heavy lifting; core does correlation + durable bus.
- **Contracts live in `.proto` files** and are generated with Buf.

```
[Mic] → voice_agent (C++ / sherpa-onnx)
          │
          │ transcript + tool-call JSON
          ▼
     Local LLM (Ollama / llama.cpp)
          │
          │ POST /v1/query/plates  or  MCP tool call
          ▼
┌─────────────────────────────────────────┐
│  voice-query-service (this repo, Go)    │
│  • HTTP :8080                           │
│  • MCP stdio / POST /mcp                │
│  • tools: plates, objects, scenes, …    │
│  • ClickHouse primary                   │
│  • optional NATS live peek              │
└─────────────────────────────────────────┘
          │
          ▼
   ClickHouse  ←  clickhouse-consumer
```

This service is **read-only**. It never publishes back into the CV pipeline.

## Quickstart

### Prerequisites

- Go 1.22+
- Buf CLI (`curl -sSL https://github.com/bufbuild/buf/releases/latest/download/buf-Linux-x86_64 -o /tmp/buf && chmod +x /tmp/buf && sudo mv /tmp/buf /usr/local/bin/`)

### Generate & tidy

```bash
make generate
make tidy
```

### Demo (no ClickHouse)

```bash
export DEMO_MODE=true
go run ./cmd/server
# → listens on :8080

curl -s http://localhost:8080/health | jq
curl -s -X POST http://localhost:8080/v1/query/plates \
  -H 'Content-Type: application/json' \
  -d '{"make_model":"Mercedes","time_window_sec":30,"limit":5}' | jq
```

Expected:

```json
[
  {
    "plate": "7ABC123",
    "confidence": 0.94,
    "track_id": "77",
    "camera_id": "cam-01",
    "vehicle_class": "car",
    "make_model": "Mercedes C-Class",
    "scene_l1": "roadway",
    "scene_l2": "driving"
  }
]
```

## Run against ClickHouse

Same env as AgentJetson core compose:

```bash
export CLICKHOUSE_HOST=localhost   # or "clickhouse" inside compose
export CLICKHOUSE_PORT=9000
export CLICKHOUSE_USER=default
export CLICKHOUSE_PASSWORD=pass
export APPLY_SCHEMA=true           # creates 002 tables if missing
export DEMO_MODE=false
go run ./cmd/server
```

## Makefile targets

| Target        | Description                                      |
|---------------|--------------------------------------------------|
| `make generate` | Buf generate → `gen/`                          |
| `make tidy`     | `go mod tidy`                                  |
| `make clean`    | Remove generated stubs                         |
| `make run`      | `go run ./cmd/server`                          |
| `make run-demo` | DEMO_MODE=true                                 |
| `make up`       | docker compose up -d --build                   |
| `make down`     | docker compose down -v                         |
| `make info`     | Print instructions                             |

## Configuration (env)

| Variable                  | Default                  | Notes                                      |
|---------------------------|--------------------------|--------------------------------------------|
| `HTTP_ADDR`               | `:8080`                  | Listen address                             |
| `DEMO_MODE`               | `false`                  | In-memory samples, no ClickHouse required  |
| `CLICKHOUSE_HOST`         | `localhost`              |                                            |
| `CLICKHOUSE_PORT`         | `9000`                   | Native protocol                            |
| `CLICKHOUSE_USER`         | `default`                |                                            |
| `CLICKHOUSE_PASSWORD`     | `pass`                   |                                            |
| `CLICKHOUSE_DATABASE`     | `default`                |                                            |
| `APPLY_SCHEMA`            | `true`                   | Create tables on startup                   |
| `NATS_URL`                | `nats://localhost:4222`  | Optional live path                         |
| `NATS_ENABLE_LIVE`        | `false`                  |                                            |
| `DEFAULT_TIME_WINDOW_SEC` | `30`                     |                                            |
| `DEFAULT_LIMIT`           | `5`                      |                                            |

## HTTP API

| Method | Path                      | Body / notes                                      |
|--------|---------------------------|---------------------------------------------------|
| GET    | `/health`                 | Service status                                    |
| GET    | `/v1/tools`               | List of tool names                                |
| POST   | `/v1/query/plates`        | `QueryRecentPlatesArgs` JSON                      |
| POST   | `/v1/query/objects`       | class / track_id / camera_id / time window        |
| POST   | `/v1/query/scenes`        | level1 / camera_id / time window                  |
| POST   | `/v1/query/detections`    | type / camera_id / time window                    |
| POST   | `/v1/query/transcripts`   | contains / time window                            |
| POST   | `/mcp`                    | `{ "tool": "query_recent_plates", "arguments": {} }` |

## MCP tools

Run with `--stdio` for IDE / Claude Desktop:

```bash
./voice-query-service --stdio
```

Registered tools:

- `query_recent_plates` — ALPR results (vehicle_class, make_model, color, scene_l1, time_window_sec, limit)
- `query_objects` — object envelopes / tracks
- `query_scenes` — SceneResult L1/L2
- `query_detections` — aggregator detections / alerts path
- `query_transcripts` — audio transcripts
- `health`

## Project layout

```
voice-query-service/
├── cmd/server/main.go          # HTTP + stdio entrypoint
├── internal/
│   ├── mcp/                    # MCP server + tool registration
│   ├── tools/                  # tool handlers
│   ├── clickhouse/             # CH client + demo store
│   ├── config/                 # env config
│   └── models/                 # shared structs
├── sql/002_envelopes.sql       # schema for cv_results, cv_objects, …
├── Dockerfile
├── docker-compose.override.yml
├── go.mod
└── README.md
```

## Schema notes

`clickhouse_consumer` writes `cv_detections` (from `cv.alert`) and `audio_transcripts`.  
`query_recent_plates` targets **`cv_results`** (capability = `'alpr'`).  

Set `APPLY_SCHEMA=true` so this service creates the 002 tables then INSERT `CapabilityResult` rows into `cv_results` from the consumer so live ALPR hits appear.
