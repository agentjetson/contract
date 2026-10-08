package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	natsv1 "github.com/agentjetson/core/gen/go/nats/v1"
	"github.com/agentjetson/core/nats-publisher/internal/config"
	"github.com/agentjetson/core/nats-publisher/internal/service"
	"github.com/agentjetson/core/pkg/natsjs"
	"google.golang.org/grpc"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()

	nc, js, err := natsjs.Connect(cfg.NATSURL, cfg.ClientName)
	if err != nil {
		slog.Error("nats", "err", err)
		os.Exit(1)
	}
	defer nc.Close()
	slog.Info("nats connected", "url", cfg.NATSURL, "client", cfg.ClientName)

	// Ensure JetStream streams (subject ownership from domain/nats-subjects.yaml).
	// CV_EVENTS includes cv.scene.> (added relative to the original C++ binary).
	if err := natsjs.EnsureAllStreams(js, nil); err != nil {
		slog.Error("ensure streams", "err", err)
		os.Exit(1)
	}

	svc := service.New(js)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		slog.Error("listen", "addr", cfg.GRPCAddr, "err", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer()
	natsv1.RegisterNatsPublisherServiceServer(grpcServer, svc)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		slog.Info("shutting down nats publisher...")
		grpcServer.GracefulStop()
	}()

	slog.Info("NATS Publisher grpc listening", "addr", cfg.GRPCAddr)
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("grpc serve", "err", err)
		os.Exit(1)
	}
}
