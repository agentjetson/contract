// Ingest server: edge → core gRPC ingress + JetStream publisher.
//
// Combined binary: accepts IngestService RPCs and owns the NATS connection.
// Also registers NatsPublisherService on the same gRPC server so specialists
// that dial the publisher proto keep working without a second process.
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"os/signal"
	"syscall"

	ingestv1 "github.com/agentjetson/core/gen/go/ingest/v1"
	natsv1 "github.com/agentjetson/core/gen/go/nats/v1"
	"github.com/agentjetson/core/ingest/internal/config"
	"github.com/agentjetson/core/ingest/internal/service"
	"github.com/agentjetson/core/pkg/natsjs"
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

	nc, js, err := natsjs.Connect(cfg.NATSURL, cfg.ClientName)
	if err != nil {
		slog.Error("nats", "err", err)
		os.Exit(1)
	}
	defer nc.Close()
	slog.Info("nats connected", "url", cfg.NATSURL, "client", cfg.ClientName)

	if err := natsjs.EnsureAllStreams(js, nil); err != nil {
		slog.Error("ensure streams", "err", err)
		os.Exit(1)
	}

	pub := service.NewPublisher(js)
	ingestSvc := service.NewIngest(pub)

	lis, err := net.Listen("tcp", cfg.GRPCAddr)
	if err != nil {
		slog.Error("listen", "addr", cfg.GRPCAddr, "err", err)
		os.Exit(1)
	}

	grpcServer := grpc.NewServer(otel.GRPCServerOption())
	ingestv1.RegisterIngestServiceServer(grpcServer, ingestSvc)
	natsv1.RegisterNatsPublisherServiceServer(grpcServer, pub)

	go func() {
		<-ctx.Done()
		slog.Info("shutting down ingest server...")
		grpcServer.GracefulStop()
	}()

	slog.Info("Ingest+Publisher grpc listening",
		"addr", cfg.GRPCAddr,
		"nats", cfg.NATSURL,
		"services", []string{"IngestService", "NatsPublisherService"},
	)
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("grpc serve", "err", err)
		os.Exit(1)
	}
}
