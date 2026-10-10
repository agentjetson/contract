// Ingest server: edge → core gRPC ingress.
//
// Go port of core/src/ingest. Thin pass-through onto NatsPublisherService;
// does not own a NATS connection itself.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	ingestv1 "github.com/agentjetson/core/gen/go/ingest/v1"
	"github.com/agentjetson/core/ingest/internal/config"
	"github.com/agentjetson/core/ingest/internal/service"
	"github.com/agentjetson/core/pkg/otel"
	"google.golang.org/grpc"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	shutdown, err := otel.Init(ctx, "ingest")
	if err != nil {
		slog.Error("otel", "err", err)
		os.Exit(1)
	}
	defer func() { _ = shutdown(context.Background()) }()

	cfg := config.Load()

	conn, natsClient, err := service.DialPublisher(cfg.PublisherAddr)
	if err != nil {
		slog.Error("dial publisher", "addr", cfg.PublisherAddr, "err", err)
		os.Exit(1)
	}
	defer conn.Close()
	slog.Info("publisher connected", "addr", cfg.PublisherAddr)

	svc := service.New(natsClient)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		slog.Error("listen", "addr", cfg.GRPCAddr, "err", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(otel.GRPCServerOption())
	ingestv1.RegisterIngestServiceServer(grpcServer, svc)

	go func() {
		<-ctx.Done()
		slog.Info("shutting down ingest server...")
		grpcServer.GracefulStop()
	}()

	slog.Info("Ingest grpc listening", "addr", cfg.GRPCAddr, "publisher", cfg.PublisherAddr)
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("grpc serve", "err", err)
		os.Exit(1)
	}
}
