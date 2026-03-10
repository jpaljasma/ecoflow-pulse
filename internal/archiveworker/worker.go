package archiveworker

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"log/slog"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/telemetrybus"
	"github.com/klauspost/compress/zstd"
	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	defaultConsumerDurable      = "archive-raw-v1"
	defaultQueueGroup           = "archive-raw"
	defaultAckWait              = 60 * time.Second
	defaultMaxAckPending        = 4096
	defaultProcessTimeout       = 3 * time.Second
	defaultDrainTimeout         = 8 * time.Second
	defaultFailureAlertWindow   = 10 * time.Minute
	defaultFailureAlertThresh   = 6
	defaultFailureAlertCooldown = 5 * time.Minute
	defaultFlushInterval        = 30 * time.Second
	defaultFlushTimeout         = 12 * time.Second
	defaultMaxRecords           = 1024
	defaultMaxBytes             = 4 * 1024 * 1024
	defaultObjectBucket         = "pulse-telemetry-raw"
	defaultObjectPrefix         = "raw"
	defaultStreamName           = "PULSE_TELEMETRY_INGEST"
	defaultDedupWindow          = 30 * time.Minute
	defaultDedupMaxEntries      = 250_000
)

type Config struct {
	SubjectConfig SubjectConfig
	StreamName    string
	Durable       string
	QueueGroup    string

	AckWait               time.Duration
	MaxAckPending         int
	ProcessTimeout        time.Duration
	DrainTimeout          time.Duration
	FailureAlertWindow    time.Duration
	FailureAlertThreshold int
	FailureAlertCooldown  time.Duration

	FlushInterval     time.Duration
	FlushTimeout      time.Duration
	MaxRecordsPerPart int
	MaxBytesPerPart   int
	ObjectBucket      string
	ObjectPrefix      string
	WriterID          string
	ZstdEncoderLevel  int
	DedupWindow       time.Duration
	DedupMaxEntries   int
}

type SubjectConfig = telemetrybus.SubjectConfig

type WorkerOption func(*Worker) error

func DefaultConfig() Config {
	return Config{
		SubjectConfig: SubjectConfig{
			Prefix:     telemetrybus.DefaultSubjectPrefix,
			ShardCount: telemetrybus.DefaultShardCount,
		},
		StreamName:            defaultStreamName,
		Durable:               defaultConsumerDurable,
		QueueGroup:            defaultQueueGroup,
		AckWait:               defaultAckWait,
		MaxAckPending:         defaultMaxAckPending,
		ProcessTimeout:        defaultProcessTimeout,
		DrainTimeout:          defaultDrainTimeout,
		FailureAlertWindow:    defaultFailureAlertWindow,
		FailureAlertThreshold: defaultFailureAlertThresh,
		FailureAlertCooldown:  defaultFailureAlertCooldown,
		FlushInterval:         defaultFlushInterval,
		FlushTimeout:          defaultFlushTimeout,
		MaxRecordsPerPart:     defaultMaxRecords,
		MaxBytesPerPart:       defaultMaxBytes,
		ObjectBucket:          defaultObjectBucket,
		ObjectPrefix:          defaultObjectPrefix,
		WriterID:              defaultWriterID(),
		ZstdEncoderLevel:      3,
		DedupWindow:           defaultDedupWindow,
		DedupMaxEntries:       defaultDedupMaxEntries,
	}
}

func (c Config) normalized() Config {
	out := c
	out.SubjectConfig = out.SubjectConfig.Normalized()
	if strings.TrimSpace(out.StreamName) == "" {
		out.StreamName = defaultStreamName
	}
	if strings.TrimSpace(out.Durable) == "" {
		out.Durable = defaultConsumerDurable
	}
	if strings.TrimSpace(out.QueueGroup) == "" {
		out.QueueGroup = defaultQueueGroup
	}
	if out.AckWait <= 0 {
		out.AckWait = defaultAckWait
	}
	if out.MaxAckPending <= 0 {
		out.MaxAckPending = defaultMaxAckPending
	}
	if out.ProcessTimeout <= 0 {
		out.ProcessTimeout = defaultProcessTimeout
	}
	if out.DrainTimeout <= 0 {
		out.DrainTimeout = defaultDrainTimeout
	}
	if out.FailureAlertWindow <= 0 {
		out.FailureAlertWindow = defaultFailureAlertWindow
	}
	if out.FailureAlertThreshold <= 0 {
		out.FailureAlertThreshold = defaultFailureAlertThresh
	}
	if out.FailureAlertCooldown <= 0 {
		out.FailureAlertCooldown = defaultFailureAlertCooldown
	}
	if out.FlushInterval <= 0 {
		out.FlushInterval = defaultFlushInterval
	}
	if out.FlushTimeout <= 0 {
		out.FlushTimeout = defaultFlushTimeout
	}
	if out.MaxRecordsPerPart <= 0 {
		out.MaxRecordsPerPart = defaultMaxRecords
	}
	if out.MaxBytesPerPart <= 0 {
		out.MaxBytesPerPart = defaultMaxBytes
	}
	if out.DedupWindow <= 0 {
		out.DedupWindow = defaultDedupWindow
	}
	if out.DedupMaxEntries <= 0 {
		out.DedupMaxEntries = defaultDedupMaxEntries
	}
	if strings.TrimSpace(out.ObjectBucket) == "" {
		out.ObjectBucket = defaultObjectBucket
	}
	if strings.TrimSpace(out.ObjectPrefix) == "" {
		out.ObjectPrefix = defaultObjectPrefix
	}
	out.WriterID = sanitizeWriterID(out.WriterID)
	if out.WriterID == "" {
		out.WriterID = defaultWriterID()
	}
	return out
}

type delivery interface {
	Subject() string
	Data() []byte
	Ack() error
	Nak() error
	Term() error
}

type natsDelivery struct {
	msg *nats.Msg
}

func (d natsDelivery) Subject() string { return d.msg.Subject }
func (d natsDelivery) Data() []byte    { return d.msg.Data }
func (d natsDelivery) Ack() error      { return d.msg.Ack() }
func (d natsDelivery) Nak() error      { return d.msg.Nak() }
func (d natsDelivery) Term() error     { return d.msg.Term() }

type segmentKey struct {
	partitionHour time.Time
	shard         uint32
}

func (k segmentKey) mapKey() string {
	return fmt.Sprintf("%d/%03d", k.partitionHour.Unix(), k.shard)
}

type archiveSegment struct {
	key               segmentKey
	part              int
	openedAt          time.Time
	buffer            bytes.Buffer
	encoder           *zstd.Encoder
	pending           []delivery
	records           int
	tsMin             int64
	tsMax             int64
	providers         map[string]struct{}
	deviceIDs         map[string]struct{}
	providerDeviceIDs map[string]struct{}
}

type archiveRecordMeta struct {
	provider         string
	deviceID         string
	providerDeviceID string
}

type Worker struct {
	log           *slog.Logger
	conn          *nats.Conn
	store         ObjectStore
	manifestStore ManifestStore
	cfg           Config

	nowFn         func() time.Time
	subscribe     func(js nats.JetStreamContext, handler nats.MsgHandler) (*nats.Subscription, error)
	segments      map[string]*archiveSegment
	partCounts    map[string]int
	failureAlerts *failureRateTracker
	deduper       *recentEnvelopeDeduper
	tracker       *telemetrybus.MsgHandlerTracker
	mu            sync.Mutex
}

func New(log *slog.Logger, conn *nats.Conn, store ObjectStore, cfg Config, options ...WorkerOption) (*Worker, error) {
	if conn == nil {
		return nil, errors.New("nats connection is required")
	}
	if store == nil {
		return nil, errors.New("object store is required")
	}
	if log == nil {
		log = slog.Default()
	}
	cfg = cfg.normalized()
	w := &Worker{
		log:        log,
		conn:       conn,
		store:      store,
		cfg:        cfg,
		nowFn:      time.Now,
		segments:   make(map[string]*archiveSegment),
		partCounts: make(map[string]int),
		deduper:    newRecentEnvelopeDeduper(cfg.DedupWindow, cfg.DedupMaxEntries),
		tracker:    telemetrybus.NewMsgHandlerTracker(),
		failureAlerts: newFailureRateTracker(
			cfg.FailureAlertWindow,
			cfg.FailureAlertThreshold,
			cfg.FailureAlertCooldown,
		),
	}
	w.subscribe = w.defaultSubscribe
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(w); err != nil {
			return nil, err
		}
	}
	return w, nil
}

func WithManifestStore(store ManifestStore) WorkerOption {
	return func(w *Worker) error {
		if w == nil {
			return errors.New("archive worker is not initialized")
		}
		w.manifestStore = store
		return nil
	}
}

func (w *Worker) Run(ctx context.Context) error {
	js, err := w.conn.JetStream()
	if err != nil {
		return fmt.Errorf("init jetstream context: %w", err)
	}
	sub, err := w.subscribe(js, w.tracker.Wrap(w.handleMessage))
	if err != nil {
		return fmt.Errorf("subscribe archive consumer: %w", err)
	}

	stopTicker := make(chan struct{})
	var tickerWG sync.WaitGroup
	tickerWG.Add(1)
	go func() {
		defer tickerWG.Done()
		ticker := time.NewTicker(w.cfg.FlushInterval)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				flushCtx, cancel := context.WithTimeout(context.Background(), w.cfg.FlushTimeout)
				if flushErr := w.flushDue(flushCtx); flushErr != nil {
					w.log.Warn("archive periodic flush failed", slog.String("error", flushErr.Error()))
				}
				cancel()
			case <-stopTicker:
				return
			case <-ctx.Done():
				return
			}
		}
	}()

	w.log.Info("archive worker running",
		slog.String("subject", telemetrybus.IngestWildcardSubject(w.cfg.SubjectConfig)),
		slog.String("stream", w.cfg.StreamName),
		slog.String("queue_group", w.cfg.QueueGroup),
		slog.String("durable", w.cfg.Durable),
		slog.String("bucket", w.cfg.ObjectBucket),
		slog.String("object_prefix", w.cfg.ObjectPrefix),
		slog.Duration("flush_interval", w.cfg.FlushInterval),
		slog.Int("max_records_per_part", w.cfg.MaxRecordsPerPart),
		slog.Int("max_bytes_per_part", w.cfg.MaxBytesPerPart),
		slog.Duration("dedup_window", w.cfg.DedupWindow),
		slog.Int("dedup_max_entries", w.cfg.DedupMaxEntries),
	)

	<-ctx.Done()
	close(stopTicker)
	tickerWG.Wait()
	w.drainSubscription(sub)

	flushCtx, cancel := context.WithTimeout(context.Background(), w.cfg.FlushTimeout)
	defer cancel()
	if err := w.flushAll(flushCtx); err != nil {
		return fmt.Errorf("flush archive segments on shutdown: %w", err)
	}
	return nil
}

func (w *Worker) defaultSubscribe(js nats.JetStreamContext, handler nats.MsgHandler) (*nats.Subscription, error) {
	return js.QueueSubscribe(
		telemetrybus.IngestWildcardSubject(w.cfg.SubjectConfig),
		w.cfg.QueueGroup,
		handler,
		nats.BindStream(strings.TrimSpace(w.cfg.StreamName)),
		nats.Durable(strings.TrimSpace(w.cfg.Durable)),
		nats.ManualAck(),
		nats.AckWait(w.cfg.AckWait),
		nats.MaxAckPending(w.cfg.MaxAckPending),
	)
}

func (w *Worker) handleMessage(msg *nats.Msg) {
	if msg == nil {
		return
	}
	procCtx, cancel := context.WithTimeout(context.Background(), w.cfg.ProcessTimeout)
	defer cancel()
	if err := w.processDelivery(procCtx, natsDelivery{msg: msg}); err != nil {
		failCount, failPerMin, spike := w.failureAlerts.Record(w.nowFn().UTC())
		w.log.Warn("archive process delivery failed",
			slog.String("error", err.Error()),
			slog.String("subject", msg.Subject),
			slog.Int("archive_failures_in_window", failCount),
			slog.Float64("archive_failures_per_min", failPerMin),
			slog.Duration("archive_failure_window", w.cfg.FailureAlertWindow),
		)
		if spike {
			w.log.Warn("archive-failure spike detected",
				slog.Int("archive_failures_in_window", failCount),
				slog.Float64("archive_failures_per_min", failPerMin),
				slog.Duration("window", w.cfg.FailureAlertWindow),
				slog.Int("threshold", w.cfg.FailureAlertThreshold),
				slog.Duration("cooldown", w.cfg.FailureAlertCooldown),
				slog.String("subject", msg.Subject),
			)
		}
	}
}

func (w *Worker) processDelivery(_ context.Context, d delivery) error {
	if d == nil {
		return nil
	}
	var env envelopev1.TelemetryEnvelope
	if err := proto.Unmarshal(d.Data(), &env); err != nil {
		if termErr := d.Term(); termErr != nil {
			w.log.Warn("archive term invalid envelope failed", slog.String("error", termErr.Error()))
		}
		return fmt.Errorf("unmarshal telemetry envelope: %w", err)
	}
	partition := envelopePartitionTime(&env, w.nowFn()).Truncate(time.Hour)
	shard := env.GetShard()
	if env.GetShardCount() == 0 || env.GetShard() >= env.GetShardCount() {
		shard = telemetrybus.ShardForDevice(strings.TrimSpace(env.GetDeviceId()), w.cfg.SubjectConfig.ShardCount)
	}
	key := segmentKey{partitionHour: partition, shard: shard}

	payload, err := proto.Marshal(&env)
	if err != nil {
		if termErr := d.Term(); termErr != nil {
			w.log.Warn("archive term envelope with marshal failure failed", slog.String("error", termErr.Error()))
		}
		return fmt.Errorf("marshal telemetry envelope for archive: %w", err)
	}

	w.mu.Lock()
	defer w.mu.Unlock()
	if dedupKey := archiveDedupKey(&env); dedupKey != "" {
		if !w.deduper.Add(w.nowFn().UTC(), dedupKey) {
			_ = d.Ack()
			return nil
		}
	}
	segment, err := w.segmentForKeyLocked(key)
	if err != nil {
		_ = d.Nak()
		return err
	}
	if err := segment.append(payload, d, w.recordTimestampUnixMilli(&env), manifestRecordMetaFromEnvelope(&env)); err != nil {
		_ = d.Nak()
		_ = w.dropSegmentLocked(segment)
		return fmt.Errorf("append envelope to archive segment: %w", err)
	}

	if segment.records >= w.cfg.MaxRecordsPerPart || segment.buffer.Len() >= w.cfg.MaxBytesPerPart {
		flushCtx, cancel := context.WithTimeout(context.Background(), w.cfg.FlushTimeout)
		err := w.flushSegmentLocked(flushCtx, segment)
		cancel()
		if err != nil {
			return err
		}
	}
	return nil
}

func (w *Worker) recordTimestampUnixMilli(env *envelopev1.TelemetryEnvelope) int64 {
	if env == nil {
		return w.nowFn().UnixMilli()
	}
	if ts := env.GetIngestedTimeUnixMs(); ts > 0 {
		return ts
	}
	if ts := env.GetObservedTimeUnixMs(); ts > 0 {
		return ts
	}
	if ts := env.GetDeviceTimeUnixMs(); ts > 0 {
		return ts
	}
	return w.nowFn().UnixMilli()
}

func (w *Worker) flushDue(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	now := w.nowFn()
	var firstErr error
	for _, segment := range w.segments {
		if now.Sub(segment.openedAt) < w.cfg.FlushInterval {
			continue
		}
		if err := w.flushSegmentLocked(ctx, segment); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (w *Worker) flushAll(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	var firstErr error
	for _, segment := range w.segments {
		if err := w.flushSegmentLocked(ctx, segment); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (w *Worker) segmentForKeyLocked(key segmentKey) (*archiveSegment, error) {
	if existing := w.segments[key.mapKey()]; existing != nil {
		return existing, nil
	}
	partPrefix := fmt.Sprintf("%d/%03d", key.partitionHour.Unix(), key.shard)
	w.partCounts[partPrefix]++
	part := w.partCounts[partPrefix]

	level := zstd.EncoderLevelFromZstd(w.cfg.ZstdEncoderLevel)
	segment := &archiveSegment{
		key:      key,
		part:     part,
		openedAt: w.nowFn(),
	}
	encoder, err := zstd.NewWriter(&segment.buffer, zstd.WithEncoderLevel(level))
	if err != nil {
		return nil, fmt.Errorf("init zstd encoder: %w", err)
	}
	segment.encoder = encoder
	w.segments[key.mapKey()] = segment
	return segment, nil
}

func (w *Worker) flushSegmentLocked(ctx context.Context, segment *archiveSegment) error {
	if segment == nil {
		return nil
	}
	defer func() {
		delete(w.segments, segment.key.mapKey())
	}()
	if segment.records == 0 {
		return nil
	}
	if err := segment.encoder.Close(); err != nil {
		ackErr := w.nakPending(segment.pending)
		if ackErr != nil {
			w.log.Warn("archive segment close failed and nak failed", slog.String("error", ackErr.Error()))
		}
		return fmt.Errorf("close zstd encoder: %w", err)
	}

	objectKey := buildArchiveObjectKey(
		w.cfg.ObjectPrefix,
		segment.key.partitionHour,
		segment.key.shard,
		segment.part,
		w.cfg.WriterID,
	)
	body := append([]byte(nil), segment.buffer.Bytes()...)
	checksum := crc32.ChecksumIEEE(body)
	req := PutObjectRequest{
		Bucket:      w.cfg.ObjectBucket,
		Key:         objectKey,
		Body:        body,
		ContentType: defaultObjectContentType,
		Metadata: map[string]string{
			"record_count":   strconv.Itoa(segment.records),
			"ts_min_unix_ms": strconv.FormatInt(segment.tsMin, 10),
			"ts_max_unix_ms": strconv.FormatInt(segment.tsMax, 10),
			"shard":          strconv.FormatUint(uint64(segment.key.shard), 10),
			"shard_count":    strconv.FormatUint(uint64(w.cfg.SubjectConfig.ShardCount), 10),
			"writer_id":      w.cfg.WriterID,
			"checksum_crc32": fmt.Sprintf("%08x", checksum),
		},
	}
	if err := w.store.PutObject(ctx, req); err != nil {
		_ = w.nakPending(segment.pending)
		return fmt.Errorf("write archive object %q: %w", objectKey, err)
	}
	if w.manifestStore != nil {
		record := ManifestRecord{
			Provider:          segmentProvider(segment.providers),
			Shard:             segment.key.shard,
			ShardCount:        w.cfg.SubjectConfig.ShardCount,
			PartitionHour:     segment.key.partitionHour,
			TSMinUnixMS:       segment.tsMin,
			TSMaxUnixMS:       segment.tsMax,
			RecordCount:       segment.records,
			ObjectBucket:      req.Bucket,
			ObjectKey:         req.Key,
			ObjectSizeBytes:   int64(len(body)),
			ContentType:       req.ContentType,
			Compression:       defaultManifestCompression,
			ChecksumCRC32:     fmt.Sprintf("%08x", checksum),
			WriterID:          w.cfg.WriterID,
			DeviceIDs:         normalizeStringSet(mapSetValues(segment.deviceIDs), false),
			ProviderDeviceIDs: normalizeStringSet(mapSetValues(segment.providerDeviceIDs), true),
		}
		if err := w.manifestStore.UpsertObjectManifest(ctx, record); err != nil {
			_ = w.nakPending(segment.pending)
			return fmt.Errorf("persist archive manifest %q: %w", objectKey, err)
		}
	}
	_ = w.ackPending(segment.pending)
	return nil
}

func (w *Worker) dropSegmentLocked(segment *archiveSegment) error {
	if segment == nil {
		return nil
	}
	delete(w.segments, segment.key.mapKey())
	if segment.encoder != nil {
		_ = segment.encoder.Close()
	}
	return nil
}

func (w *Worker) drainSubscription(sub *nats.Subscription) {
	if sub == nil {
		return
	}
	if err := sub.Unsubscribe(); err != nil && !errors.Is(err, nats.ErrBadSubscription) {
		w.log.Warn("archive unsubscribe failed", slog.String("error", err.Error()))
	}
	if !w.tracker.WaitForIdle(w.cfg.DrainTimeout) {
		w.log.Warn("archive handler drain timeout")
	}
}

func (s *archiveSegment) append(payload []byte, d delivery, recordTs int64, meta archiveRecordMeta) error {
	if len(payload) == 0 {
		return errors.New("archive payload is empty")
	}
	var sizePrefix [binary.MaxVarintLen64]byte
	n := binary.PutUvarint(sizePrefix[:], uint64(len(payload)))
	if _, err := s.encoder.Write(sizePrefix[:n]); err != nil {
		return fmt.Errorf("write record length prefix: %w", err)
	}
	if _, err := s.encoder.Write(payload); err != nil {
		return fmt.Errorf("write record body: %w", err)
	}
	s.records++
	if s.records == 1 {
		s.tsMin = recordTs
		s.tsMax = recordTs
	} else {
		if recordTs < s.tsMin {
			s.tsMin = recordTs
		}
		if recordTs > s.tsMax {
			s.tsMax = recordTs
		}
	}
	if provider := strings.ToLower(strings.TrimSpace(meta.provider)); provider != "" {
		if s.providers == nil {
			s.providers = make(map[string]struct{})
		}
		s.providers[provider] = struct{}{}
	}
	if deviceID := strings.TrimSpace(meta.deviceID); deviceID != "" {
		if s.deviceIDs == nil {
			s.deviceIDs = make(map[string]struct{})
		}
		s.deviceIDs[deviceID] = struct{}{}
	}
	if providerDeviceID := strings.TrimSpace(meta.providerDeviceID); providerDeviceID != "" {
		if s.providerDeviceIDs == nil {
			s.providerDeviceIDs = make(map[string]struct{})
		}
		s.providerDeviceIDs[providerDeviceID] = struct{}{}
	}
	s.pending = append(s.pending, d)
	return nil
}

func manifestRecordMetaFromEnvelope(env *envelopev1.TelemetryEnvelope) archiveRecordMeta {
	if env == nil {
		return archiveRecordMeta{}
	}
	provider := ""
	providerDeviceID := ""
	if labels := env.GetLabels(); len(labels) > 0 {
		provider = strings.TrimSpace(labels["provider"])
		providerDeviceID = strings.TrimSpace(labels["provider_device_id"])
	}
	if providerDeviceID == "" {
		providerDeviceID = strings.TrimSpace(env.GetEcoflowSn())
	}
	return archiveRecordMeta{
		provider:         provider,
		deviceID:         strings.TrimSpace(env.GetDeviceId()),
		providerDeviceID: providerDeviceID,
	}
}

func segmentProvider(providers map[string]struct{}) string {
	if len(providers) == 0 {
		return defaultManifestProvider
	}
	if len(providers) == 1 {
		for provider := range providers {
			return provider
		}
	}
	return "mixed"
}

func mapSetValues(values map[string]struct{}) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	return out
}

func (w *Worker) ackPending(deliveries []delivery) error {
	var firstErr error
	for _, d := range deliveries {
		if d == nil {
			continue
		}
		if err := d.Ack(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (w *Worker) nakPending(deliveries []delivery) error {
	var firstErr error
	for _, d := range deliveries {
		if d == nil {
			continue
		}
		if err := d.Nak(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func defaultWriterID() string {
	hostname, err := os.Hostname()
	if err != nil || strings.TrimSpace(hostname) == "" {
		hostname = "worker"
	}
	return sanitizeWriterID(fmt.Sprintf("%s-%d", hostname, os.Getpid()))
}
