package ingestworker

import (
	"context"
	"errors"
	"sync"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
)

const (
	defaultPublishQueueSize      = 256
	defaultPublishWorkers        = 1
	defaultPublishEnqueueTimeout = 2 * time.Second
)

type asyncEnvelopePublisher struct {
	publisher telemetrybus.EnvelopePublisher
	jobs      chan *envelopev1.TelemetryEnvelope
	errors    chan error
	cancel    context.CancelFunc

	enqueueTimeout time.Duration

	once sync.Once
	wg   sync.WaitGroup
}

func newAsyncEnvelopePublisher(
	parent context.Context,
	publisher telemetrybus.EnvelopePublisher,
	queueSize int,
	workers int,
	enqueueTimeout time.Duration,
) *asyncEnvelopePublisher {
	if queueSize <= 0 {
		queueSize = defaultPublishQueueSize
	}
	if workers <= 0 {
		workers = defaultPublishWorkers
	}
	if enqueueTimeout <= 0 {
		enqueueTimeout = defaultPublishEnqueueTimeout
	}

	ctx, cancel := context.WithCancel(parent)
	out := &asyncEnvelopePublisher{
		publisher:      publisher,
		jobs:           make(chan *envelopev1.TelemetryEnvelope, queueSize),
		errors:         make(chan error, queueSize),
		cancel:         cancel,
		enqueueTimeout: enqueueTimeout,
	}

	out.wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer out.wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case env, ok := <-out.jobs:
					if !ok {
						return
					}
					if err := telemetrybus.PublishEnvelope(ctx, out.publisher, env); err != nil {
						if errors.Is(err, context.Canceled) {
							return
						}
						select {
						case out.errors <- err:
						default:
						}
						continue
					}
				}
			}
		}()
	}
	return out
}

func (p *asyncEnvelopePublisher) Publish(ctx context.Context, envelope *envelopev1.TelemetryEnvelope) error {
	if p == nil {
		return errors.New("async envelope publisher is required")
	}
	if envelope == nil {
		return errors.New("envelope is required")
	}

	timer := time.NewTimer(p.enqueueTimeout)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return errors.New("enqueue telemetry envelope timeout")
	case p.jobs <- envelope:
		return nil
	}
}

func (p *asyncEnvelopePublisher) Errors() <-chan error {
	if p == nil {
		return nil
	}
	return p.errors
}

func (p *asyncEnvelopePublisher) Close() error {
	if p == nil {
		return nil
	}
	p.once.Do(func() {
		close(p.jobs)
		p.wg.Wait()
		p.cancel()
	})
	return nil
}
