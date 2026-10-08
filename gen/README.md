# Generated Go protobufs

Run from the contract repo root:

```bash
make generate   # requires buf CLI: https://buf.build/docs/installation
```

This writes `gen/go/**` from `proto/` via `buf.gen.yaml`.

`clickhouse-consumer` imports:

- `github.com/agentjetson/contract/gen/go/detection/v1`
- `github.com/agentjetson/contract/gen/go/scene/v1`
- `github.com/agentjetson/contract/gen/go/audio/v1`

Minimal stubs may be committed for CI; always re-run `make generate` after proto changes.
