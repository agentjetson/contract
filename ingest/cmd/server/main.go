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

	"github.com/agentjetson/contract/ingest/internal/config"
	"github.com/agentjetson/contract/ingest/internal/service"
	ingestv1 "github.com/agentjetson/contract/gen/go/ingest/v1"
	"google.golang.org/grpc"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

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

	grpcServer := grpc.NewServer()
	ingestv1.RegisterIngestServiceServer(grpcServer, svc)

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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
