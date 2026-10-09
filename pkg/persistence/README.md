# pkg/persistence

Shared ClickHouse client for AgentJetson core services.

**Schema is never created here.** Apply DDL with `make schema` from the
core root (`seed/sql/002_tables.sql` + `003_views.sql`).

## API

```go
ch, err := persistence.Open(persistence.Config{
    Host: "localhost", Port: 9000,
    User: "default", Password: "pass", Database: "default",
})
defer ch.Close()

_ = ch.InsertObjects(ctx, []persistence.ObjectRow{...})
_ = ch.InsertResults(ctx, []persistence.ResultRow{...})
_ = ch.InsertScenes(ctx, []persistence.SceneRow{...})
_ = ch.InsertDetections(ctx, []persistence.DetectionRow{...})
_ = ch.InsertTranscripts(ctx, []persistence.TranscriptRow{...})
_ = ch.InsertObjectMeta(ctx, persistence.ObjectMetaRow{...})

// Advanced / query-service:
conn := ch.Conn() // driver.Conn for SELECT against query_* views
```

## Consumers

| Service | Uses |
|---------|------|
| clickhouse-consumer | InsertObjects / Results / Scenes / Detections / Transcripts |
| object-storage | InsertObjectMeta |
| query-service | Conn() + SELECTs (no ApplySchema) |
