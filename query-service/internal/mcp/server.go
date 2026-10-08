package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/agentjetson/voice-query-service/internal/models"
	"github.com/agentjetson/voice-query-service/internal/tools"
)

// ToolNames returns the registered tool names.
func ToolNames() []string {
	return []string{
		"query_recent_plates",
		"query_objects",
		"query_scenes",
		"query_detections",
		"query_transcripts",
		"health",
	}
}

// ServeStdio runs a minimal JSON-line MCP-style server over stdin/stdout.
// Each request is one JSON object per line:
//
//	{"tool":"query_recent_plates","arguments":{"make_model":"Mercedes","time_window_sec":30}}
//
// Response is one JSON object per line:
//
//	{"ok":true,"result":[...]}  or  {"ok":false,"error":"..."}
//
// This keeps the C++ voice_agent and local LLM integration simple without
// pulling in an external MCP SDK. Swap for mark3labs/mcp-go later if needed.
func ServeStdio(t *tools.Server) error {
	slog.Info("MCP stdio mode (JSON-line protocol)", "tools", ToolNames())
	scanner := bufio.NewScanner(os.Stdin)
	// allow large payloads
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var req struct {
			Tool      string          `json:"tool"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(line, &req); err != nil {
			writeStdioErr(fmt.Sprintf("invalid JSON: %v", err))
			continue
		}
		result, err := dispatch(context.Background(), t, req.Tool, req.Arguments)
		if err != nil {
			writeStdioErr(err.Error())
			continue
		}
		writeStdioOK(result)
	}
	if err := scanner.Err(); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func dispatch(ctx context.Context, t *tools.Server, tool string, args json.RawMessage) (any, error) {
	switch tool {
	case "query_recent_plates":
		var a models.QueryRecentPlatesArgs
		_ = json.Unmarshal(args, &a)
		return t.QueryRecentPlates(ctx, a)
	case "query_objects":
		var a models.QueryObjectsArgs
		_ = json.Unmarshal(args, &a)
		return t.QueryObjects(ctx, a)
	case "query_scenes":
		var a models.QueryScenesArgs
		_ = json.Unmarshal(args, &a)
		return t.QueryScenes(ctx, a)
	case "query_detections":
		var a models.QueryAlertsArgs
		_ = json.Unmarshal(args, &a)
		return t.QueryDetections(ctx, a)
	case "query_transcripts":
		var a models.QueryTranscriptsArgs
		_ = json.Unmarshal(args, &a)
		return t.QueryTranscripts(ctx, a)
	case "health":
		return t.Health(ctx), nil
	default:
		return nil, fmt.Errorf("unknown tool: %s", tool)
	}
}

func writeStdioOK(result any) {
	out := map[string]any{"ok": true, "result": result}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}

func writeStdioErr(msg string) {
	out := map[string]any{"ok": false, "error": msg}
	b, _ := json.Marshal(out)
	fmt.Println(string(b))
}
