package service

import (
	"context"
	"log/slog"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"

	commonv1 "github.com/agentjetson/core/gen/go/common/v1"
	natsv1 "github.com/agentjetson/core/gen/go/nats/v1"
	"github.com/agentjetson/core/pkg/otel"
)

// Publisher implements nats.v1.NatsPublisherServiceServer.
// Shared by the combined ingest binary so specialists can still dial the
// publisher surface (same process, no extra hop when co-located).
type Publisher struct {
	natsv1.UnimplementedNatsPublisherServiceServer
	js nats.JetStreamContext
}

func NewPublisher(js nats.JetStreamContext) *Publisher {
	return &Publisher{js: js}
}

const (
	defaultAlertSubject      = "cv.alert"
	defaultTranscriptSubject = "audio.transcript"
)

// publish injects W3C trace context into NATS headers and publishes via JetStream.
func (p *Publisher) publish(ctx context.Context, subject string, payload []byte) (seq uint64, err error) {
	msg := &nats.Msg{
		Subject: subject,
		Data:    payload,
		Header:  make(nats.Header),
	}
	ctx, span := otel.StartProducerSpan(ctx, "ingest", "nats.publish "+subject, msg)
	defer span.End()

	ack, err := p.js.PublishMsg(msg, nats.Context(ctx))
	if err != nil {
		otel.RecordError(span, err)
		return 0, err
	}
	if ack != nil {
		return ack.Sequence, nil
	}
	return 0, nil
}

func setErr(code, msg string) *commonv1.DomainError {
	return &commonv1.DomainError{Code: code, Message: msg}
}

func (p *Publisher) PublishAlert(ctx context.Context, req *natsv1.PublishAlertRequest) (*natsv1.PublishAlertResponse, error) {
	resp := &natsv1.PublishAlertResponse{}
	if req.GetAlert() == nil {
		resp.Published = false
		resp.Error = setErr("INVALID_ARGUMENT", "alert is required")
		return resp, nil
	}
	subject := req.GetSubject()
	if subject == "" {
		subject = defaultAlertSubject
	}
	payload, err := proto.Marshal(req.GetAlert())
	if err != nil {
		resp.Published = false
		resp.Error = setErr("SERIALIZE_FAILED", "failed to serialize Alert protobuf")
		return resp, nil
	}
	seq, err := p.publish(ctx, subject, payload)
	if err != nil {
		slog.ErrorContext(ctx, "publish alert", "subject", subject, "err", err)
		resp.Published = false
		resp.Error = setErr("PUBLISH_FAILED", err.Error())
		return resp, nil
	}
	slog.InfoContext(ctx, "published alert",
		"subject", subject, "seq", seq,
		"frame", req.GetAlert().GetFrameId(),
		"dets", len(req.GetAlert().GetDetections()),
		"bytes", len(payload),
	)
	resp.Published = true
	resp.Subject = subject
	resp.Sequence = seq
	return resp, nil
}

func (p *Publisher) PublishTranscript(ctx context.Context, req *natsv1.PublishTranscriptRequest) (*natsv1.PublishTranscriptResponse, error) {
	resp := &natsv1.PublishTranscriptResponse{}
	if req.GetTranscript() == nil {
		resp.Published = false
		resp.Error = setErr("INVALID_ARGUMENT", "transcript is required")
		return resp, nil
	}
	subject := req.GetSubject()
	if subject == "" {
		subject = defaultTranscriptSubject
	}
	payload, err := proto.Marshal(req.GetTranscript())
	if err != nil {
		resp.Published = false
		resp.Error = setErr("SERIALIZE_FAILED", "failed to serialize Transcript protobuf")
		return resp, nil
	}
	seq, err := p.publish(ctx, subject, payload)
	if err != nil {
		slog.ErrorContext(ctx, "publish transcript", "subject", subject, "err", err)
		resp.Published = false
		resp.Error = setErr("PUBLISH_FAILED", err.Error())
		return resp, nil
	}
	resp.Published = true
	resp.Subject = subject
	resp.Sequence = seq
	return resp, nil
}

func (p *Publisher) PublishObject(ctx context.Context, req *natsv1.PublishObjectRequest) (*natsv1.PublishObjectResponse, error) {
	resp := &natsv1.PublishObjectResponse{}
	obj := req.GetObject()
	if obj == nil {
		resp.Published = false
		resp.Error = setErr("INVALID_ARGUMENT", "object is required")
		return resp, nil
	}
	subject := req.GetSubject()
	if subject == "" {
		cls := obj.GetClassName()
		if cls == "" {
			cls = "unknown"
		}
		subject = "cv.object." + cls
	}
	payload, err := proto.Marshal(obj)
	if err != nil {
		resp.Published = false
		resp.Error = setErr("SERIALIZE_FAILED", "failed to serialize ObjectEnvelope protobuf")
		return resp, nil
	}
	seq, err := p.publish(ctx, subject, payload)
	if err != nil {
		slog.ErrorContext(ctx, "publish object", "subject", subject, "err", err)
		resp.Published = false
		resp.Error = setErr("PUBLISH_FAILED", err.Error())
		return resp, nil
	}
	slog.InfoContext(ctx, "published object",
		"subject", subject, "seq", seq,
		"frame", obj.GetFrameId(), "track", obj.GetTrackId(),
		"class", obj.GetClassName(), "crop_bytes", len(obj.GetCropJpeg()),
	)
	resp.Published = true
	resp.Subject = subject
	resp.Sequence = seq
	return resp, nil
}

func (p *Publisher) PublishResult(ctx context.Context, req *natsv1.PublishResultRequest) (*natsv1.PublishResultResponse, error) {
	resp := &natsv1.PublishResultResponse{}
	res := req.GetResult()
	if res == nil {
		resp.Published = false
		resp.Error = setErr("INVALID_ARGUMENT", "result is required")
		return resp, nil
	}
	subject := req.GetSubject()
	if subject == "" {
		cap := res.GetCapability()
		if cap == "" {
			cap = "unknown"
		}
		subject = "cv.result." + cap
	}
	payload, err := proto.Marshal(res)
	if err != nil {
		resp.Published = false
		resp.Error = setErr("SERIALIZE_FAILED", "failed to serialize CapabilityResult protobuf")
		return resp, nil
	}
	seq, err := p.publish(ctx, subject, payload)
	if err != nil {
		slog.ErrorContext(ctx, "publish result", "subject", subject, "err", err)
		resp.Published = false
		resp.Error = setErr("PUBLISH_FAILED", err.Error())
		return resp, nil
	}
	slog.InfoContext(ctx, "published result",
		"subject", subject, "seq", seq,
		"frame", res.GetFrameId(), "track", res.GetTrackId(),
		"cap", res.GetCapability(), "ocr", res.GetOcrText(),
	)
	resp.Published = true
	resp.Subject = subject
	resp.Sequence = seq
	return resp, nil
}

func (p *Publisher) PublishScene(ctx context.Context, req *natsv1.PublishSceneRequest) (*natsv1.PublishSceneResponse, error) {
	resp := &natsv1.PublishSceneResponse{}
	sc := req.GetScene()
	if sc == nil {
		resp.Published = false
		resp.Error = setErr("INVALID_ARGUMENT", "scene is required")
		return resp, nil
	}
	subject := req.GetSubject()
	if subject == "" {
		lvl := sc.GetLevel1()
		if lvl == "" {
			lvl = "result"
		}
		subject = "cv.scene." + lvl
	}
	payload, err := proto.Marshal(sc)
	if err != nil {
		resp.Published = false
		resp.Error = setErr("SERIALIZE_FAILED", "failed to serialize SceneResult protobuf")
		return resp, nil
	}
	seq, err := p.publish(ctx, subject, payload)
	if err != nil {
		slog.ErrorContext(ctx, "publish scene", "subject", subject, "err", err)
		resp.Published = false
		resp.Error = setErr("PUBLISH_FAILED", err.Error())
		return resp, nil
	}
	slog.InfoContext(ctx, "published scene",
		"subject", subject, "seq", seq,
		"level1", sc.GetLevel1(), "bytes", len(payload),
	)
	resp.Published = true
	resp.Subject = subject
	resp.Sequence = seq
	return resp, nil
}

var _ natsv1.NatsPublisherServiceServer = (*Publisher)(nil)
