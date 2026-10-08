# Generated Go protobufs

**Required for `clickhouse-consumer` and any Go code that imports `gen/go/...`.**

```bash
# From contract repo root (requires buf: https://buf.build/docs/installation)
make generate
```

Writes `gen/go/**` from `proto/` via `buf.gen.yaml`.

Then ensure the module stub exists:

```bash
cat > gen/go/go.mod <<'EOF'
module github.com/agentjetson/core/gen/go

go 1.22

require google.golang.org/protobuf v1.35.1
EOF
```

`clickhouse-consumer` imports:

- `github.com/agentjetson/core/gen/go/detection/v1`
- `github.com/agentjetson/core/gen/go/scene/v1`
- `github.com/agentjetson/core/gen/go/audio/v1`

Docker builds run `buf generate` inside the image (see `clickhouse-consumer/Dockerfile`),
so images do not depend on committed stubs. For local `go build`, always run `make generate` first.

Do not commit incomplete hand-written stubs — Unmarshal will fail without real descriptors.
