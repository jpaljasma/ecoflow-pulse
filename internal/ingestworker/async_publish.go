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
	asyncPublishPollInterval     = time.Millisecond
	asyncPublishCloseTimeoutCap  = 100 * time.Millisecond
)

var errAsyncPublisherClosed = errors.New("async envelope publisher is closed")

type asyncEnvelopePublisher struct {
	publisher telemetrybus.EnvelopePublisher
	jobs      chan *envelopev1.TelemetryEnvelope
	errors    chan error
	cancel    context.CancelFunc
	done      chan struct{}

	enqueueTimeout time.Duration

	mu     sync.RWMutex
	closed bool
	once   sync.Once
	wg     sync.WaitGroup
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
		done:           make(chan struct{}),
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
	poll := time.NewTicker(asyncPublishPollInterval)
	defer poll.Stop()

	for {
		p.mu.RLock()
		if p.closed {
			p.mu.RUnlock()
			return errAsyncPublisherClosed
		}
		select {
		case p.jobs <- envelope:
			p.mu.RUnlock()
			return nil
		default:
			p.mu.RUnlock()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
			return errors.New("enqueue telemetry envelope timeout")
		case <-p.done:
			return errAsyncPublisherClosed
		case <-poll.C:
		}
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
		p.mu.Lock()
		p.closed = true
		close(p.done)
		close(p.jobs)
		p.mu.Unlock()
		drained := make(chan struct{})
		go func() {
			p.wg.Wait()
			close(drained)
		}()
		closeTimeout := p.enqueueTimeout
		if closeTimeout <= 0 || closeTimeout > asyncPublishCloseTimeoutCap {
			closeTimeout = asyncPublishCloseTimeoutCap
		}
		select {
		case <-drained:
			p.cancel()
		case <-time.After(closeTimeout):
			p.cancel()
			select {
			case <-drained:
			case <-time.After(closeTimeout):
			}
		}
	})
	return nil
}
