package telemetrybus

import (
	"fmt"
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

type ingestQueueSubscriber interface {
	QueueSubscribe(string, string, nats.MsgHandler, ...nats.SubOpt) (*nats.Subscription, error)
}

// QueueSubscribeIngest subscribes to the ingest stream without replaying the
// entire retained stream if the durable consumer ever has to be recreated.
func QueueSubscribeIngest(js ingestQueueSubscriber, cfg IngestQueueSubscribeConfig, handler nats.MsgHandler) (*nats.Subscription, error) {
	sub, err := queueSubscribeIngestCreateOrBind(js, cfg, handler)
	if err == nil {
		return sub, nil
	}
	if !isDeliverPolicyConfigDrift(err) {
		return nil, err
	}
	sub, bindErr := queueSubscribeIngestExistingDurable(js, cfg, handler)
	if bindErr != nil {
		return nil, fmt.Errorf("bind existing durable after deliver policy mismatch (%v): %w", err, bindErr)
	}
	return sub, nil
}

func queueSubscribeIngestCreateOrBind(js ingestQueueSubscriber, cfg IngestQueueSubscribeConfig, handler nats.MsgHandler) (*nats.Subscription, error) {
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

func queueSubscribeIngestExistingDurable(js ingestQueueSubscriber, cfg IngestQueueSubscribeConfig, handler nats.MsgHandler) (*nats.Subscription, error) {
	return js.QueueSubscribe(
		IngestWildcardSubject(cfg.SubjectConfig),
		strings.TrimSpace(cfg.QueueGroup),
		handler,
		nats.Bind(strings.TrimSpace(cfg.StreamName), strings.TrimSpace(cfg.Durable)),
		nats.ManualAck(),
	)
}

func isDeliverPolicyConfigDrift(err error) bool {
	return err != nil && strings.Contains(err.Error(), "configuration requests deliver policy")
}
