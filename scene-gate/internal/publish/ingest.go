package publish

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/types/known/timestamppb"

	ingestv1 "github.com/agentjetson/core/gen/go/ingest/v1"
	scenev1 "github.com/agentjetson/core/gen/go/scene/v1"
	"github.com/agentjetson/core/pkg/otel"
	"github.com/agentjetson/core/scene-gate/internal/decision"
)

// IngestClient publishes SceneResult via gRPC IngestScene.
type IngestClient struct {
	conn   *grpc.ClientConn
	client ingestv1.IngestServiceClient
}

func DialIngest(addr string) (*IngestClient, error) {
	opts := append(otel.GRPCDialOptions(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, fmt.Errorf("dial ingest %s: %w", addr, err)
	}
	return &IngestClient{
		conn:   conn,
		client: ingestv1.NewIngestServiceClient(conn),
	}, nil
}

func (c *IngestClient) Close() {
	if c != nil && c.conn != nil {
		_ = c.conn.Close()
	}
}

// PublishScene converts a gate Result into SceneResult and sends IngestScene.
func (c *IngestClient) PublishScene(ctx context.Context, r decision.Result) error {
	if r.Action != decision.EmitNow {
		return nil
	}
	scene := &scenev1.SceneResult{
		FrameId:           0,
		Timestamp:         timestamppb.New(time.Now().UTC()),
		Source:            r.SourceID,
		Level1:            r.L1,
		Level2:            r.L2,
		Level1Confidence:  r.Confidence,
		Level2Confidence:  r.Confidence,
		Specialists:       r.Specialists,
		Backend:           r.Backend,
		TemporalRequested: false,
		Notes:             r.Reason,
	}
	resp, err := c.client.IngestScene(ctx, &ingestv1.IngestSceneRequest{Scene: scene})
	if err != nil {
		return fmt.Errorf("IngestScene: %w", err)
	}
	if !resp.GetAccepted() {
		msg := "rejected"
		if resp.GetError() != nil {
			msg = resp.GetError().GetMessage()
		}
		return fmt.Errorf("IngestScene not accepted: %s", msg)
	}
	slog.InfoContext(ctx, "emitted SceneResult",
		"source", r.SourceID,
		"l1", r.L1,
		"l2", r.L2,
		"backend", r.Backend,
		"specialists", r.Specialists,
		"reason", r.Reason,
		"msg_id", resp.GetMessageId(),
	)
	return nil
}
