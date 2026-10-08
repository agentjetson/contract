// Demo / scaffolding entrypoint for the Buf-aligned voice-query-service.
//
// When DEMO_MODE=true this serves in-memory sample data so the HTTP contract
// can be exercised without ClickHouse. Merge the real cmd/server + internal/*
// from https://github.com/agentjetson/voice-query-service for production.
package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"
)

func main() {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	demo := os.Getenv("DEMO_MODE") == "true" || os.Getenv("DEMO_MODE") == ""

	mux := http.NewServeMux()
	mux.HandleFunc("/health", handleHealth(demo))
	mux.HandleFunc("/v1/tools", handleTools)
	mux.HandleFunc("/v1/query/plates", handleQueryPlates(demo))
	mux.HandleFunc("/v1/query/objects", handleQueryObjects(demo))
	mux.HandleFunc("/v1/query/scenes", handleQueryScenes(demo))
	mux.HandleFunc("/v1/query/detections", handleQueryDetections(demo))
	mux.HandleFunc("/v1/query/transcripts", handleQueryTranscripts(demo))
	mux.HandleFunc("/mcp", handleMCP(demo))

	log.Printf("voice-query-service listening on %s (DEMO_MODE=%v)", addr, demo)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func handleHealth(demo bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{
			"status":        "ok",
			"demo_mode":     demo,
			"clickhouse_ok": !demo,
			"nats_ok":       false,
			"version":       "scaffolding-0.1",
		})
	}
}

func handleTools(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, []string{
		"query_recent_plates",
		"query_objects",
		"query_scenes",
		"query_detections",
		"query_transcripts",
		"health",
	})
}

func handleQueryPlates(demo bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			MakeModel      string `json:"make_model"`
			VehicleClass   string `json:"vehicle_class"`
			Color          string `json:"color"`
			SceneL1        string `json:"scene_l1"`
			CameraID       string `json:"camera_id"`
			TimeWindowSec  int    `json:"time_window_sec"`
			Limit          int    `json:"limit"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		if !demo {
			writeJSON(w, http.StatusOK, []any{})
			return
		}

		// Sample hit matching the README expected response.
		results := []map[string]any{
			{
				"plate":         "7ABC123",
				"confidence":    0.94,
				"track_id":      "77",
				"camera_id":     "cam-01",
				"vehicle_class": "car",
				"make_model":    "Mercedes C-Class",
				"color":         "silver",
				"scene_l1":      "roadway",
				"scene_l2":      "driving",
				"observed_at":   time.Now().UTC().Format(time.RFC3339),
			},
		}

		// Optional filter: if make_model is set and doesn't match, return empty.
		if req.MakeModel != "" {
			matched := false
			for _, row := range results {
				if mm, ok := row["make_model"].(string); ok {
					if containsFold(mm, req.MakeModel) {
						matched = true
						break
					}
				}
			}
			if !matched {
				results = []map[string]any{}
			}
		}

		writeJSON(w, http.StatusOK, results)
	}
}

func handleQueryObjects(demo bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !demo {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]any{
			{
				"track_id":    "77",
				"class":       "car",
				"camera_id":   "cam-01",
				"confidence":  0.91,
				"scene_l1":    "roadway",
				"scene_l2":    "driving",
				"observed_at": time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

func handleQueryScenes(demo bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !demo {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]any{
			{
				"camera_id":   "cam-01",
				"level1":      "roadway",
				"level2":      "driving",
				"confidence":  0.88,
				"observed_at": time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

func handleQueryDetections(demo bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !demo {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]any{
			{
				"id":          "det-001",
				"type":        "alpr",
				"camera_id":   "cam-01",
				"track_id":    "77",
				"confidence":  0.94,
				"summary":     "Mercedes C-Class plate 7ABC123",
				"observed_at": time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

func handleQueryTranscripts(demo bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if !demo {
			writeJSON(w, http.StatusOK, []any{})
			return
		}
		writeJSON(w, http.StatusOK, []map[string]any{
			{
				"id":          "tr-001",
				"text":        "AJ, check the license plate of the Mercedes that just passed",
				"camera_id":   "cam-01",
				"confidence":  0.96,
				"observed_at": time.Now().UTC().Format(time.RFC3339),
			},
		})
	}
}

func handleMCP(demo bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Tool      string         `json:"tool"`
			Arguments map[string]any `json:"arguments"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)

		// Minimal MCP façade: route to the same demo data.
		switch req.Tool {
		case "query_recent_plates", "plates":
			handleQueryPlates(demo)(w, r)
		case "query_objects", "objects":
			handleQueryObjects(demo)(w, r)
		case "query_scenes", "scenes":
			handleQueryScenes(demo)(w, r)
		case "query_detections", "detections":
			handleQueryDetections(demo)(w, r)
		case "query_transcripts", "transcripts":
			handleQueryTranscripts(demo)(w, r)
		case "health":
			handleHealth(demo)(w, r)
		default:
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "unknown tool: " + req.Tool,
			})
		}
	}
}

func containsFold(s, substr string) bool {
	if substr == "" {
		return true
	}
	// simple case-insensitive contains without importing strings for brevity
	ls, lsub := toLower(s), toLower(substr)
	return len(ls) >= len(lsub) && (ls == lsub || indexOf(ls, lsub) >= 0)
}

func toLower(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
