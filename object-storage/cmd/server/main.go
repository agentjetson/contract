package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/agentjetson/object-storage/internal/config"
	"github.com/agentjetson/object-storage/internal/metadata"
	"github.com/agentjetson/object-storage/internal/server"
	"github.com/agentjetson/object-storage/internal/storage"
)

func main() {
	cfg := config.FromEnv()
	log.Printf("object-storage starting backend=%s demo=%v addr=%s", cfg.Backend, cfg.DemoMode, cfg.GRPCAddr)

	backend, err := buildBackend(cfg)
	if err != nil {
		log.Fatalf("backend: %v", err)
	}
	ctx := context.Background()
	if err := backend.EnsureBucket(ctx); err != nil {
		log.Fatalf("ensure bucket: %v", err)
	}

	meta := metadata.NewMemoryStore()
	svc := server.New(backend, meta, cfg.Bucket)

	// Lightweight HTTP surface for health + simple put/get demos.
	// Full gRPC surface is generated from proto/storage/v1 once Buf is wired.
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		h, _ := svc.Health(r.Context(), &server.HealthRequest{})
		writeJSON(w, h)
	})
	mux.HandleFunc("/v1/objects", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPost:
			handlePut(w, r, svc)
		case http.MethodGet:
			handleList(w, r, svc)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})
	mux.HandleFunc("/v1/objects/", func(w http.ResponseWriter, r *http.Request) {
		id := r.URL.Path[len("/v1/objects/"):]
		if id == "" {
			http.Error(w, "object_id required", http.StatusBadRequest)
			return
		}
		switch r.Method {
		case http.MethodGet:
			handleGet(w, r, svc, id)
		case http.MethodDelete:
			handleDelete(w, r, svc, id)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
	})

	httpAddr := cfg.GRPCAddr
	// Prefer a distinct HTTP port when GRPC_ADDR looks like pure gRPC.
	if os.Getenv("HTTP_ADDR") != "" {
		httpAddr = os.Getenv("HTTP_ADDR")
	} else {
		// Default HTTP on 8081 so it does not collide with query-service :8080.
		httpAddr = "0.0.0.0:8081"
	}

	httpSrv := &http.Server{Addr: httpAddr, Handler: mux}
	go func() {
		log.Printf("HTTP listening on %s", httpAddr)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	// Placeholder gRPC listener — binds the configured port so compose health
	// checks and future generated stubs can attach without changing env vars.
	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			log.Printf("gRPC listen warning (HTTP still up): %v", err)
			return
		}
		log.Printf("gRPC port reserved on %s (wire generated stubs here)", cfg.GRPCAddr)
		// Block until process exit; real grpc.Server.Serve goes here after buf generate.
		<-ctx.Done()
		_ = lis.Close()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("shutting down…")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
}

func buildBackend(cfg config.Config) (storage.Backend, error) {
	switch cfg.Backend {
	case "filesystem":
		return storage.NewFilesystemBackend(cfg.FSRoot, cfg.Bucket), nil
	case "minio", "s3":
		return storage.NewMinioBackend(
			cfg.Endpoint, cfg.AccessKey, cfg.SecretKey,
			cfg.Bucket, cfg.Region, cfg.UseSSL, cfg.ForcePathStyle,
		)
	default:
		return nil, fmt.Errorf("unknown STORAGE_BACKEND %q", cfg.Backend)
	}
}

// ---------------------------------------------------------------------------
// HTTP handlers (demo / integration surface)
// ---------------------------------------------------------------------------

func handlePut(w http.ResponseWriter, r *http.Request, svc *server.Service) {
	kindStr := r.URL.Query().Get("kind")
	source := r.URL.Query().Get("source")
	contentType := r.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "application/octet-stream"
	}
	if source == "" {
		source = "unknown"
	}
	data, err := io.ReadAll(io.LimitReader(r.Body, 64<<20)) // 64 MiB soft limit for unary
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	kind := server.ObjectKindOther
	switch kindStr {
	case "audio_transcript", "1":
		kind = server.ObjectKindAudioTranscript
	case "camera_recording", "2":
		kind = server.ObjectKindCameraRecording
	case "frame_snapshot", "3":
		kind = server.ObjectKindFrameSnapshot
	case "crop", "4":
		kind = server.ObjectKindCrop
	case "annotated_clip", "5":
		kind = server.ObjectKindAnnotatedClip
	}
	resp, err := svc.PutObject(r.Context(), &server.PutObjectRequest{
		Kind:        kind,
		Source:      source,
		Timestamp:   time.Now().UTC(),
		ContentType: contentType,
		Data:        data,
		Labels:      map[string]string{"via": "http"},
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if !resp.Accepted {
		w.WriteHeader(http.StatusBadRequest)
	}
	writeJSON(w, resp)
}

func handleList(w http.ResponseWriter, r *http.Request, svc *server.Service) {
	source := r.URL.Query().Get("source")
	resp, err := svc.ListObjects(r.Context(), &server.ListObjectsRequest{
		Source: source,
		Limit:  100,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp)
}

func handleGet(w http.ResponseWriter, r *http.Request, svc *server.Service, id string) {
	if r.URL.Query().Get("meta") == "1" {
		resp, _ := svc.GetObjectMeta(r.Context(), &server.GetObjectMetaRequest{ObjectID: id})
		writeJSON(w, resp)
		return
	}
	rc, meta, err := svc.GetObjectStream(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	defer rc.Close()
	if meta != nil && meta.ContentType != "" {
		w.Header().Set("Content-Type", meta.ContentType)
	}
	w.Header().Set("X-Object-Id", id)
	_, _ = io.Copy(w, rc)
}

func handleDelete(w http.ResponseWriter, r *http.Request, svc *server.Service, id string) {
	hard := r.URL.Query().Get("hard") == "1"
	resp, err := svc.DeleteObject(r.Context(), &server.DeleteObjectRequest{
		ObjectID:   id,
		HardDelete: hard,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, resp)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(v)
}
