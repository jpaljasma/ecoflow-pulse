package telemetrybus

import (
	"strings"
	"time"

	"github.com/nats-io/nats.go"
)

// IngestQueueSubscribeConfig captures the common durable queue subscription
// settings used by workers that consume normalized ingest envelopes.
type IngestQueueSubscribeConfig struct {
	SubjectConfig SubjectConfig
	StreamName    string
	Durable       string
	QueueGroup    string
	AckWait       time.Duration
	MaxAckPending int
}

// QueueSubscribeIngest subscribes to the ingest stream without replaying the
// entire retained stream if the durable consumer ever has to be recreated.
func QueueSubscribeIngest(js nats.JetStreamContext, cfg IngestQueueSubscribeConfig, handler nats.MsgHandler) (*nats.Subscription, error) {
	return js.QueueSubscribe(
		IngestWildcardSubject(cfg.SubjectConfig),
		strings.TrimSpace(cfg.QueueGroup),
		handler,
		nats.BindStream(strings.TrimSpace(cfg.StreamName)),
		nats.Durable(strings.TrimSpace(cfg.Durable)),
		nats.ManualAck(),
		nats.AckWait(cfg.AckWait),
		nats.MaxAckPending(cfg.MaxAckPending),
		nats.DeliverNew(),
	)
}
