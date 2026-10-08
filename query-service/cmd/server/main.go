package main

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentjetson/voice-query-service/internal/clickhouse"
	"github.com/agentjetson/voice-query-service/internal/config"
	"github.com/agentjetson/voice-query-service/internal/mcp"
	"github.com/agentjetson/voice-query-service/internal/models"
	"github.com/agentjetson/voice-query-service/internal/tools"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	cfg := config.Load()

	ch, err := clickhouse.New(cfg)
	if err != nil {
		slog.Error("clickhouse", "err", err)
		os.Exit(1)
	}
	defer ch.Close()

	tsrv := tools.New(cfg, ch)

	if os.Getenv("MCP_STDIO") == "true" {
		if err := mcp.ServeStdio(tsrv); err != nil {
			slog.Error("mcp stdio", "err", err)
			os.Exit(1)
		}
		return
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, tsrv.Health(r.Context()))
	})
	mux.HandleFunc("/v1/tools", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, mcp.ToolNames())
	})
	mux.HandleFunc("/v1/query/plates", handlePost(func(ctx context.Context, body []byte) (any, error) {
		var a models.QueryRecentPlatesArgs
		_ = json.Unmarshal(body, &a)
		return tsrv.QueryRecentPlates(ctx, a)
	}))
	mux.HandleFunc("/v1/query/objects", handlePost(func(ctx context.Context, body []byte) (any, error) {
		var a models.QueryObjectsArgs
		_ = json.Unmarshal(body, &a)
		return tsrv.QueryObjects(ctx, a)
	}))
	mux.HandleFunc("/v1/query/scenes", handlePost(func(ctx context.Context, body []byte) (any, error) {
		var a models.QueryScenesArgs
		_ = json.Unmarshal(body, &a)
		return tsrv.QueryScenes(ctx, a)
	}))
	mux.HandleFunc("/v1/query/detections", handlePost(func(ctx context.Context, body []byte) (any, error) {
		var a models.QueryAlertsArgs
		_ = json.Unmarshal(body, &a)
		return tsrv.QueryDetections(ctx, a)
	}))
	mux.HandleFunc("/v1/query/transcripts", handlePost(func(ctx context.Context, body []byte) (any, error) {
		var a models.QueryTranscriptsArgs
		_ = json.Unmarshal(body, &a)
		return tsrv.QueryTranscripts(ctx, a)
	}))
	mux.HandleFunc("/mcp", handleMCP(tsrv))

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		shctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shctx)
	}()

	slog.Info("voice-query-service listening",
		"addr", cfg.HTTPAddr,
		"demo_mode", cfg.DemoMode,
		"clickhouse", cfg.ClickHouseHost,
	)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		slog.Error("http", "err", err)
		os.Exit(1)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func handlePost(fn func(ctx context.Context, body []byte) (any, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var body []byte
		if r.Body != nil {
			body, _ = io.ReadAll(io.LimitReader(r.Body, 1<<20))
		}
		ctx, cancel := context.WithTimeout(r.Context(), config.DefaultTimeout())
		defer cancel()
		result, err := fn(ctx, body)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}

func handleMCP(tsrv *tools.Server) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		ctx, cancel := context.WithTimeout(r.Context(), config.DefaultTimeout())
		defer cancel()

		var (
			result any
			err    error
		)
		switch req.Tool {
		case "query_recent_plates", "plates":
			var a models.QueryRecentPlatesArgs
			_ = json.Unmarshal(req.Arguments, &a)
			result, err = tsrv.QueryRecentPlates(ctx, a)
		case "query_objects", "objects":
			var a models.QueryObjectsArgs
			_ = json.Unmarshal(req.Arguments, &a)
			result, err = tsrv.QueryObjects(ctx, a)
		case "query_scenes", "scenes":
			var a models.QueryScenesArgs
			_ = json.Unmarshal(req.Arguments, &a)
			result, err = tsrv.QueryScenes(ctx, a)
		case "query_detections", "detections":
			var a models.QueryAlertsArgs
			_ = json.Unmarshal(req.Arguments, &a)
			result, err = tsrv.QueryDetections(ctx, a)
		case "query_transcripts", "transcripts":
			var a models.QueryTranscriptsArgs
			_ = json.Unmarshal(req.Arguments, &a)
			result, err = tsrv.QueryTranscripts(ctx, a)
		case "health":
			result = tsrv.Health(ctx)
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown tool: " + req.Tool})
			return
		}
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, result)
	}
}
