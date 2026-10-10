package service

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/status"

	commonv1 "github.com/agentjetson/core/gen/go/common/v1"
	ingestv1 "github.com/agentjetson/core/gen/go/ingest/v1"
	natsv1 "github.com/agentjetson/core/gen/go/nats/v1"
	"github.com/agentjetson/core/pkg/otel"
)

// Ingest implements ingest.v1.IngestServiceServer.
// Thin pass-through onto NatsPublisherService — edge → core ingress.
type Ingest struct {
	ingestv1.UnimplementedIngestServiceServer
	nats natsv1.NatsPublisherServiceClient
}

func New(nats natsv1.NatsPublisherServiceClient) *Ingest {
	return &Ingest{nats: nats}
}

func formatSeq(seq uint64) string {
	return fmt.Sprintf("js-%d", seq)
}

func domainErr(code, msg string) *commonv1.DomainError {
	return &commonv1.DomainError{Code: code, Message: msg}
}

// ---- IngestAlert ----

func (s *Ingest) IngestAlert(ctx context.Context, req *ingestv1.IngestAlertRequest) (*ingestv1.IngestAlertResponse, error) {
	resp := &ingestv1.IngestAlertResponse{}
	if req.GetAlert() == nil {
		resp.Accepted = false
		resp.Error = domainErr("INVALID_ARGUMENT", "alert is required")
		return resp, nil
	}

	pubResp, err := s.nats.PublishAlert(ctx, &natsv1.PublishAlertRequest{
		Alert:   req.GetAlert(),
		Subject: "cv.alert",
	})
	if err != nil {
		slog.ErrorContext(ctx, "nats publisher call failed", "rpc", "PublishAlert", "err", err)
		return nil, status.Errorf(codes.Internal, "nats publisher unavailable: %v", err)
	}
	if !pubResp.GetPublished() {
		msg := "publish rejected"
		if pubResp.GetError() != nil {
			msg = pubResp.GetError().GetMessage()
		}
		resp.Accepted = false
		resp.Error = domainErr("PUBLISH_FAILED", msg)
		return resp, nil
	}

	a := req.GetAlert()
	slog.InfoContext(ctx, "ingested alert",
		"frame", a.GetFrameId(),
		"dets", len(a.GetDetections()),
		"latency_ms", a.GetE2ELatencyMs(),
		"seq", pubResp.GetSequence(),
	)
	resp.Accepted = true
	resp.MessageId = formatSeq(pubResp.GetSequence())
	return resp, nil
}

// ---- IngestTranscript ----

func (s *Ingest) IngestTranscript(ctx context.Context, req *ingestv1.IngestTranscriptRequest) (*ingestv1.IngestTranscriptResponse, error) {
	resp := &ingestv1.IngestTranscriptResponse{}
	if req.GetTranscript() == nil {
		resp.Accepted = false
		resp.Error = domainErr("INVALID_ARGUMENT", "transcript is required")
		return resp, nil
	}

	pubResp, err := s.nats.PublishTranscript(ctx, &natsv1.PublishTranscriptRequest{
		Transcript: req.GetTranscript(),
		Subject:    "audio.transcript",
	})
	if err != nil {
		slog.ErrorContext(ctx, "nats publisher call failed", "rpc", "PublishTranscript", "err", err)
		return nil, status.Errorf(codes.Internal, "nats publisher unavailable: %v", err)
	}
	if !pubResp.GetPublished() {
		msg := "publish rejected"
		if pubResp.GetError() != nil {
			msg = pubResp.GetError().GetMessage()
		}
		resp.Accepted = false
		resp.Error = domainErr("PUBLISH_FAILED", msg)
		return resp, nil
	}

	t := req.GetTranscript()
	text := t.GetText()
	if len(text) > 80 {
		text = text[:80]
	}
	slog.InfoContext(ctx, "ingested transcript",
		"source", t.GetSource(),
		"final", t.GetIsFinal(),
		"latency_ms", t.GetE2ELatencyMs(),
		"text", text,
		"seq", pubResp.GetSequence(),
	)
	resp.Accepted = true
	resp.MessageId = formatSeq(pubResp.GetSequence())
	return resp, nil
}

// ---- IngestObject ----

func (s *Ingest) IngestObject(ctx context.Context, req *ingestv1.IngestObjectRequest) (*ingestv1.IngestObjectResponse, error) {
	resp := &ingestv1.IngestObjectResponse{}
	if req.GetObject() == nil {
		resp.Accepted = false
		resp.Error = domainErr("INVALID_ARGUMENT", "object is required")
		return resp, nil
	}

	// subject left empty → publisher derives cv.object.<class_name>
	pubResp, err := s.nats.PublishObject(ctx, &natsv1.PublishObjectRequest{
		Object: req.GetObject(),
	})
	if err != nil {
		slog.ErrorContext(ctx, "nats publisher call failed", "rpc", "PublishObject", "err", err)
		return nil, status.Errorf(codes.Internal, "nats publisher unavailable: %v", err)
	}
	if !pubResp.GetPublished() {
		msg := "publish rejected"
		if pubResp.GetError() != nil {
			msg = pubResp.GetError().GetMessage()
		}
		resp.Accepted = false
		resp.Error = domainErr("PUBLISH_FAILED", msg)
		return resp, nil
	}

	o := req.GetObject()
	slog.InfoContext(ctx, "ingested object",
		"frame", o.GetFrameId(),
		"track", o.GetTrackId(),
		"class", o.GetClassName(),
		"subject", pubResp.GetSubject(),
		"seq", pubResp.GetSequence(),
	)
	resp.Accepted = true
	resp.MessageId = formatSeq(pubResp.GetSequence())
	return resp, nil
}

// ---- IngestResult ----

func (s *Ingest) IngestResult(ctx context.Context, req *ingestv1.IngestResultRequest) (*ingestv1.IngestResultResponse, error) {
	resp := &ingestv1.IngestResultResponse{}
	if req.GetResult() == nil {
		resp.Accepted = false
		resp.Error = domainErr("INVALID_ARGUMENT", "result is required")
		return resp, nil
	}

	pubResp, err := s.nats.PublishResult(ctx, &natsv1.PublishResultRequest{
		Result: req.GetResult(),
	})
	if err != nil {
		slog.ErrorContext(ctx, "nats publisher call failed", "rpc", "PublishResult", "err", err)
		return nil, status.Errorf(codes.Internal, "nats publisher unavailable: %v", err)
	}
	if !pubResp.GetPublished() {
		msg := "publish rejected"
		if pubResp.GetError() != nil {
			msg = pubResp.GetError().GetMessage()
		}
		resp.Accepted = false
		resp.Error = domainErr("PUBLISH_FAILED", msg)
		return resp, nil
	}

	r := req.GetResult()
	slog.InfoContext(ctx, "ingested result",
		"frame", r.GetFrameId(),
		"track", r.GetTrackId(),
		"cap", r.GetCapability(),
		"ocr", r.GetOcrText(),
		"subject", pubResp.GetSubject(),
		"seq", pubResp.GetSequence(),
	)
	resp.Accepted = true
	resp.MessageId = formatSeq(pubResp.GetSequence())
	return resp, nil
}

// ---- IngestScene ----
// Present in the proto; the C++ binary did not implement it yet.
// Edge scene-router / temporal-classifier call this path.

func (s *Ingest) IngestScene(ctx context.Context, req *ingestv1.IngestSceneRequest) (*ingestv1.IngestSceneResponse, error) {
	resp := &ingestv1.IngestSceneResponse{}
	if req.GetScene() == nil {
		resp.Accepted = false
		resp.Error = domainErr("INVALID_ARGUMENT", "scene is required")
		return resp, nil
	}

	pubResp, err := s.nats.PublishScene(ctx, &natsv1.PublishSceneRequest{
		Scene: req.GetScene(),
	})
	if err != nil {
		slog.ErrorContext(ctx, "nats publisher call failed", "rpc", "PublishScene", "err", err)
		return nil, status.Errorf(codes.Internal, "nats publisher unavailable: %v", err)
	}
	if !pubResp.GetPublished() {
		msg := "publish rejected"
		if pubResp.GetError() != nil {
			msg = pubResp.GetError().GetMessage()
		}
		resp.Accepted = false
		resp.Error = domainErr("PUBLISH_FAILED", msg)
		return resp, nil
	}

	sc := req.GetScene()
	slog.InfoContext(ctx, "ingested scene",
		"level1", sc.GetLevel1(),
		"subject", pubResp.GetSubject(),
		"seq", pubResp.GetSequence(),
	)
	resp.Accepted = true
	resp.MessageId = formatSeq(pubResp.GetSequence())
	return resp, nil
}

// Ensure the generated Unimplemented server is satisfied at compile time.
var _ ingestv1.IngestServiceServer = (*Ingest)(nil)

// DialPublisher creates an insecure gRPC client to NatsPublisherService.
// Uses otel.GRPCDialOptions so W3C trace context propagates to the publisher.
// Caller owns the connection lifecycle (Close via the returned *grpc.ClientConn).
func DialPublisher(addr string) (*grpc.ClientConn, natsv1.NatsPublisherServiceClient, error) {
	opts := append(otel.GRPCDialOptions(), grpc.WithTransportCredentials(insecure.NewCredentials()))
	conn, err := grpc.NewClient(addr, opts...)
	if err != nil {
		return nil, nil, fmt.Errorf("dial nats-publisher at %s: %w", addr, err)
	}
	return conn, natsv1.NewNatsPublisherServiceClient(conn), nil
}
