package service

import (
	"context"
	"fmt"
	"log/slog"

	ingestv1 "github.com/agentjetson/core/gen/go/ingest/v1"
	natsv1 "github.com/agentjetson/core/gen/go/nats/v1"
	commonv1 "github.com/agentjetson/core/gen/go/common/v1"
)

// Ingest implements ingest.v1.IngestServiceServer.
// Publishes directly via the in-process Publisher (owns NATS) — no gRPC hop.
type Ingest struct {
	ingestv1.UnimplementedIngestServiceServer
	pub *Publisher
}

func NewIngest(pub *Publisher) *Ingest {
	return &Ingest{pub: pub}
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

	pubResp, err := s.pub.PublishAlert(ctx, &natsv1.PublishAlertRequest{
		Alert:   req.GetAlert(),
		Subject: "cv.alert",
	})
	if err != nil {
		slog.ErrorContext(ctx, "publish failed", "rpc", "PublishAlert", "err", err)
		resp.Accepted = false
		resp.Error = domainErr("PUBLISH_FAILED", err.Error())
		return resp, nil
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

	pubResp, err := s.pub.PublishTranscript(ctx, &natsv1.PublishTranscriptRequest{
		Transcript: req.GetTranscript(),
		Subject:    "audio.transcript",
	})
	if err != nil {
		slog.ErrorContext(ctx, "publish failed", "rpc", "PublishTranscript", "err", err)
		resp.Accepted = false
		resp.Error = domainErr("PUBLISH_FAILED", err.Error())
		return resp, nil
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

	pubResp, err := s.pub.PublishObject(ctx, &natsv1.PublishObjectRequest{
		Object: req.GetObject(),
	})
	if err != nil {
		slog.ErrorContext(ctx, "publish failed", "rpc", "PublishObject", "err", err)
		resp.Accepted = false
		resp.Error = domainErr("PUBLISH_FAILED", err.Error())
		return resp, nil
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

	pubResp, err := s.pub.PublishResult(ctx, &natsv1.PublishResultRequest{
		Result: req.GetResult(),
	})
	if err != nil {
		slog.ErrorContext(ctx, "publish failed", "rpc", "PublishResult", "err", err)
		resp.Accepted = false
		resp.Error = domainErr("PUBLISH_FAILED", err.Error())
		return resp, nil
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

func (s *Ingest) IngestScene(ctx context.Context, req *ingestv1.IngestSceneRequest) (*ingestv1.IngestSceneResponse, error) {
	resp := &ingestv1.IngestSceneResponse{}
	if req.GetScene() == nil {
		resp.Accepted = false
		resp.Error = domainErr("INVALID_ARGUMENT", "scene is required")
		return resp, nil
	}

	pubResp, err := s.pub.PublishScene(ctx, &natsv1.PublishSceneRequest{
		Scene: req.GetScene(),
	})
	if err != nil {
		slog.ErrorContext(ctx, "publish failed", "rpc", "PublishScene", "err", err)
		resp.Accepted = false
		resp.Error = domainErr("PUBLISH_FAILED", err.Error())
		return resp, nil
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

var _ ingestv1.IngestServiceServer = (*Ingest)(nil)
