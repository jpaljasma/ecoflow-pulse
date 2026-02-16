package main

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
)

func newReconnectAttemptState(initialBackoff, maxBackoff time.Duration) *reconnectAttemptState {
	if initialBackoff <= 0 {
		initialBackoff = 500 * time.Millisecond
	}
	if maxBackoff < initialBackoff {
		maxBackoff = initialBackoff
	}
	return &reconnectAttemptState{
		initialBackoff: initialBackoff,
		maxBackoff:     maxBackoff,
		currentBackoff: initialBackoff,
	}
}

func (s *reconnectAttemptState) currentAttempt() int {
	if s == nil {
		return 1
	}
	return s.failureCount + 1
}

func (s *reconnectAttemptState) registerFailure(jitterFactor float64) (attempt int, wait time.Duration) {
	if s == nil {
		return 1, 0
	}
	attempt = s.currentAttempt()
	wait = applyJitter(s.currentBackoff, jitterFactor)
	s.failureCount++
	if s.currentBackoff < s.maxBackoff {
		s.currentBackoff *= 2
		if s.currentBackoff > s.maxBackoff {
			s.currentBackoff = s.maxBackoff
		}
	}
	return attempt, wait
}

func (s *reconnectAttemptState) reset() {
	if s == nil {
		return
	}
	s.failureCount = 0
	s.currentBackoff = s.initialBackoff
}

func connectMQTTSessionWithRetry(
	ctx context.Context,
	service *ecoflow.GeneralInfoService,
	deviceSN string,
	topicOverride string,
	keepAlive time.Duration,
	connectTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	logger *slog.Logger,
	runLog *mqttOutputLogger,
	onRetryEvent func(mqttConnectRetryEvent),
) (*ecoflowmqtt.Subscriber, string, string, error) {
	if service == nil {
		return nil, "", "", errors.New("general info service is nil")
	}
	state := newReconnectAttemptState(500*time.Millisecond, 15*time.Second)
	const jitterFactor = 0.25

	for {
		if ctx.Err() != nil {
			return nil, "", "", ctx.Err()
		}

		cert, _, err := service.GetMQTTCertification(ctx)
		if err != nil {
			attempt, wait := state.registerFailure(jitterFactor)
			logger.Warn(
				"get mqtt certification failed; retrying",
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()),
				slog.Duration("retry_in", wait),
			)
			runLog.Printf("mqtt_cert_fetch_failed attempt=%d error=%q retry_in=%s", attempt, err.Error(), wait.String())
			if sleepErr := sleepContext(ctx, wait); sleepErr != nil {
				return nil, "", "", sleepErr
			}
			continue
		}
		if strings.TrimSpace(cert.URL) == "" || strings.TrimSpace(cert.Port) == "" {
			attempt, wait := state.registerFailure(jitterFactor)
			err := errors.New("mqtt certification missing url/port")
			logger.Warn(
				"mqtt certification invalid; retrying",
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()),
				slog.Duration("retry_in", wait),
			)
			runLog.Printf("mqtt_cert_invalid attempt=%d error=%q retry_in=%s", attempt, err.Error(), wait.String())
			if sleepErr := sleepContext(ctx, wait); sleepErr != nil {
				return nil, "", "", sleepErr
			}
			continue
		}

		topic := topicOverride
		if topic == "" {
			topic = fmt.Sprintf("/open/%s/%s/quota", cert.CertificateAccount, deviceSN)
		}
		address := fmt.Sprintf("%s:%s", cert.URL, cert.Port)

		subscriber, err := ecoflowmqtt.NewSubscriber(ecoflowmqtt.Config{
			Address:        address,
			Username:       cert.CertificateAccount,
			Password:       cert.CertificatePassword,
			ClientID:       buildClientID(deviceSN),
			KeepAlive:      keepAlive,
			ConnectTimeout: connectTimeout,
			ReadTimeout:    readTimeout,
			WriteTimeout:   writeTimeout,
		})
		if err != nil {
			// Configuration error is not recoverable by retrying.
			return nil, "", "", fmt.Errorf("init mqtt subscriber: %w", err)
		}

		connectErr := subscriber.Connect(ctx)
		if connectErr == nil {
			subscribeErr := subscriber.Subscribe(ctx, topic, 0)
			if subscribeErr == nil {
				state.reset()
				if onRetryEvent != nil {
					onRetryEvent(mqttConnectRetryEvent{
						Connected: true,
						Topic:     topic,
						Broker:    address,
					})
				}
				return subscriber, topic, address, nil
			}
			connectErr = fmt.Errorf("subscribe mqtt topic: %w", subscribeErr)
		}
		_ = subscriber.Close()

		attempt, wait := state.registerFailure(jitterFactor)
		authRejected := isMQTTConnectRejected(connectErr)
		if onRetryEvent != nil {
			onRetryEvent(mqttConnectRetryEvent{
				Connected:    false,
				AuthRejected: authRejected,
				Attempt:      attempt,
				RetryIn:      wait,
				Error:        connectErr,
				Topic:        topic,
				Broker:       address,
			})
		}
		if authRejected {
			logger.Warn(
				"mqtt connect rejected by broker; refreshing certification and retrying",
				slog.Int("attempt", attempt),
				slog.String("error", connectErr.Error()),
				slog.Duration("retry_in", wait),
			)
			runLog.Printf(
				"mqtt_connect_rejected_retrying attempt=%d error=%q retry_in=%s",
				attempt,
				connectErr.Error(),
				wait.String(),
			)
		} else {
			logger.Warn(
				"mqtt connect/subscribe failed; retrying",
				slog.Int("attempt", attempt),
				slog.String("error", connectErr.Error()),
				slog.Duration("retry_in", wait),
			)
			runLog.Printf(
				"mqtt_connect_failed_retrying attempt=%d error=%q retry_in=%s",
				attempt,
				connectErr.Error(),
				wait.String(),
			)
		}
		if sleepErr := sleepContext(ctx, wait); sleepErr != nil {
			return nil, "", "", sleepErr
		}
	}
}

func isMQTTConnectRejected(err error) bool {
	if err == nil {
		return false
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "connect rejected") ||
		strings.Contains(lower, "return code=5") ||
		strings.Contains(lower, "not authorized")
}

func emitMQTTSessionEvent(events chan<- mqttSessionEvent, event mqttSessionEvent) {
	if events == nil {
		return
	}
	select {
	case events <- event:
	default:
	}
}

func runMQTTSessionLoop(
	ctx context.Context,
	service *ecoflow.GeneralInfoService,
	deviceSN string,
	topicOverride string,
	keepAlive time.Duration,
	connectTimeout time.Duration,
	readTimeout time.Duration,
	writeTimeout time.Duration,
	idleReconnectAfter time.Duration,
	logger *slog.Logger,
	runLog *mqttOutputLogger,
	queue chan ecoflowmqtt.Message,
	queueStats *mqttQueueStats,
	events chan<- mqttSessionEvent,
) {
	for {
		subscriber, topic, _, err := connectMQTTSessionWithRetry(
			ctx,
			service,
			deviceSN,
			topicOverride,
			keepAlive,
			connectTimeout,
			readTimeout,
			writeTimeout,
			logger,
			runLog,
			func(retryEvent mqttConnectRetryEvent) {
				if retryEvent.Connected {
					emitMQTTSessionEvent(events, mqttSessionEvent{
						Kind:   mqttSessionEventConnected,
						Topic:  retryEvent.Topic,
						Broker: retryEvent.Broker,
					})
					return
				}
				emitMQTTSessionEvent(events, mqttSessionEvent{
					Kind:         mqttSessionEventConnectFailure,
					AuthRejected: retryEvent.AuthRejected,
					Attempt:      retryEvent.Attempt,
					RetryIn:      retryEvent.RetryIn,
					Error:        retryEvent.Error,
					Topic:        retryEvent.Topic,
					Broker:       retryEvent.Broker,
				})
			},
		)
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return
			}
			emitMQTTSessionEvent(events, mqttSessionEvent{
				Kind:  mqttSessionEventFatal,
				Error: err,
			})
			return
		}

		reconnectState := newReconnectAttemptState(500*time.Millisecond, 15*time.Second)
		readErr := readMQTTIntoQueue(
			ctx,
			subscriber,
			topic,
			logger,
			runLog,
			reconnectState,
			queue,
			queueStats,
			idleReconnectAfter,
		)
		_ = subscriber.Close()
		if errors.Is(ctx.Err(), context.Canceled) || errors.Is(readErr, context.Canceled) {
			return
		}
		if readErr != nil {
			emitMQTTSessionEvent(events, mqttSessionEvent{
				Kind:         mqttSessionEventDisconnected,
				AuthRejected: isMQTTConnectRejected(readErr),
				Error:        readErr,
				Topic:        topic,
			})
			runLog.Printf("mqtt_session_disconnected error=%q", readErr.Error())
			continue
		}
	}
}

func reconnectSubscriber(
	ctx context.Context,
	subscriber *ecoflowmqtt.Subscriber,
	topic string,
	logger *slog.Logger,
	runLog *mqttOutputLogger,
	state *reconnectAttemptState,
) error {
	if state == nil {
		state = newReconnectAttemptState(500*time.Millisecond, 15*time.Second)
	}
	const jitterFactor = 0.25

	for {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		attempt := state.currentAttempt()
		_ = subscriber.Disconnect()

		connectErr := subscriber.Connect(ctx)
		if connectErr == nil {
			subscribeErr := subscriber.Subscribe(ctx, topic, 0)
			if subscribeErr == nil {
				state.reset()
				logger.Info(
					"ecoflow mqtt reconnected",
					slog.Int("attempt", attempt),
					slog.String("topic", topic),
				)
				runLog.Printf("mqtt_reconnected attempt=%d topic=%s", attempt, topic)
				return nil
			}
			connectErr = fmt.Errorf("subscribe: %w", subscribeErr)
		}

		if isMQTTConnectRejected(connectErr) {
			logger.Warn(
				"ecoflow mqtt reconnect auth rejected; restarting session with fresh certification",
				slog.Int("attempt", attempt),
				slog.String("error", connectErr.Error()),
			)
			runLog.Printf("mqtt_reconnect_auth_rejected attempt=%d error=%q", attempt, connectErr.Error())
			return fmt.Errorf("reconnect auth rejected: %w", connectErr)
		}

		attempt, wait := state.registerFailure(jitterFactor)
		logger.Warn(
			"ecoflow mqtt reconnect attempt failed",
			slog.Int("attempt", attempt),
			slog.String("error", connectErr.Error()),
			slog.Duration("retry_in", wait),
		)
		runLog.Printf("mqtt_reconnect_failed attempt=%d error=%q retry_in=%s", attempt, connectErr.Error(), wait.String())
		if err := sleepContext(ctx, wait); err != nil {
			return err
		}
	}
}

func enqueueMQTTMessageDropOldest(
	ctx context.Context,
	queue chan ecoflowmqtt.Message,
	message ecoflowmqtt.Message,
	stats *mqttQueueStats,
) (enqueued bool, droppedOldest bool) {
	select {
	case <-ctx.Done():
		return false, false
	case queue <- message:
		return true, false
	default:
	}

	select {
	case <-ctx.Done():
		return false, false
	case <-queue:
		droppedOldest = true
		if stats != nil {
			stats.droppedOldest.Add(1)
		}
	default:
		return false, false
	}

	select {
	case <-ctx.Done():
		return false, droppedOldest
	case queue <- message:
		return true, droppedOldest
	default:
		return false, droppedOldest
	}
}

func readMQTTIntoQueue(
	ctx context.Context,
	subscriber *ecoflowmqtt.Subscriber,
	topic string,
	logger *slog.Logger,
	runLog *mqttOutputLogger,
	reconnectState *reconnectAttemptState,
	queue chan ecoflowmqtt.Message,
	queueStats *mqttQueueStats,
	idleReconnectAfter time.Duration,
) error {
	for {
		readCtx := ctx
		var readCancel context.CancelFunc
		if idleReconnectAfter > 0 {
			readCtx, readCancel = context.WithTimeout(ctx, idleReconnectAfter)
		}
		msg, err := subscriber.ReadMessage(readCtx)
		if readCancel != nil {
			readCancel()
		}
		if err != nil {
			if errors.Is(ctx.Err(), context.Canceled) || errors.Is(err, context.Canceled) {
				return ctx.Err()
			}
			if idleReconnectAfter > 0 && errors.Is(err, context.DeadlineExceeded) {
				logger.Warn(
					"mqtt idle timeout; reconnecting",
					slog.Duration("idle_timeout", idleReconnectAfter),
				)
				runLog.Printf("mqtt_idle_timeout_reconnecting idle_timeout=%s", idleReconnectAfter.String())
				if reconnectErr := reconnectSubscriber(ctx, subscriber, topic, logger, runLog, reconnectState); reconnectErr != nil {
					return fmt.Errorf("reconnect mqtt subscriber: %w", reconnectErr)
				}
				continue
			}
			if !isReconnectableReadError(err) {
				return fmt.Errorf("read mqtt message: %w", err)
			}
			logger.Warn(
				"mqtt read failed; reconnecting",
				slog.String("error", err.Error()),
			)
			runLog.Printf("mqtt_read_error_reconnecting error=%q", err.Error())
			if reconnectErr := reconnectSubscriber(ctx, subscriber, topic, logger, runLog, reconnectState); reconnectErr != nil {
				return fmt.Errorf("reconnect mqtt subscriber: %w", reconnectErr)
			}
			continue
		}

		enqueued, dropped := enqueueMQTTMessageDropOldest(ctx, queue, msg, queueStats)
		if dropped {
			droppedCount := uint64(0)
			if queueStats != nil {
				droppedCount = queueStats.droppedOldest.Load()
			}
			if droppedCount == 1 || droppedCount%25 == 0 {
				logger.Warn(
					"mqtt ingress queue dropped oldest message",
					slog.Uint64("dropped_oldest", droppedCount),
					slog.Int("queue_depth", len(queue)),
					slog.Int("queue_capacity", cap(queue)),
				)
				runLog.Printf(
					"mqtt_queue_drop_oldest dropped_oldest=%d queue_depth=%d queue_capacity=%d",
					droppedCount,
					len(queue),
					cap(queue),
				)
			}
		}
		if !enqueued && ctx.Err() != nil {
			return ctx.Err()
		}
	}
}

func isReconnectableReadError(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, io.EOF) || errors.Is(err, net.ErrClosed) {
		return true
	}
	var netErr net.Error
	if errors.As(err, &netErr) {
		return true
	}
	lower := strings.ToLower(err.Error())
	return strings.Contains(lower, "eof") ||
		strings.Contains(lower, "broken pipe") ||
		strings.Contains(lower, "connection reset") ||
		strings.Contains(lower, "closed network connection")
}

func sleepContext(ctx context.Context, duration time.Duration) error {
	if duration <= 0 {
		return nil
	}
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func startQuitKeyListener(cancel context.CancelFunc, logger *slog.Logger, runLog *mqttOutputLogger) (func(), error) {
	stdin := os.Stdin
	if stdin == nil {
		return func() {}, errors.New("stdin is nil")
	}
	if !isCharacterDevice(stdin) {
		return func() {}, nil
	}

	restore, err := setupTerminalForSingleKeyInput(stdin)
	if err != nil {
		return func() {}, err
	}

	var stopOnce sync.Once
	stop := func() {
		stopOnce.Do(func() {
			restore()
		})
	}

	go func() {
		buf := make([]byte, 1)
		for {
			n, readErr := stdin.Read(buf)
			if readErr != nil {
				// Expected during shutdown if stdin closes or terminal detaches.
				if !errors.Is(readErr, io.EOF) && !errors.Is(readErr, os.ErrClosed) {
					logger.Debug("keyboard quit listener stopped", slog.String("error", readErr.Error()))
				}
				return
			}
			if n == 0 {
				continue
			}
			switch buf[0] {
			case 'q', 'Q':
				runLog.Printf("session_stop reason=keyboard_q")
				stop()
				cancel()
				return
			}
		}
	}()

	return stop, nil
}

func isCharacterDevice(file *os.File) bool {
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}

func applyJitter(base time.Duration, factor float64) time.Duration {
	if base <= 0 || factor <= 0 {
		return base
	}
	var randomByte [1]byte
	if _, err := rand.Read(randomByte[:]); err != nil {
		return base
	}
	unit := float64(randomByte[0])/255.0*2 - 1
	delta := time.Duration(float64(base) * factor * unit)
	value := base + delta
	if value < 100*time.Millisecond {
		return 100 * time.Millisecond
	}
	return value
}
