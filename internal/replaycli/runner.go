package replaycli

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"google.golang.org/protobuf/proto"
)

type Runner struct {
	log          *slog.Logger
	manifest     ManifestStore
	objectReader ObjectReader
	publisher    ReplayPublisher
}

func NewRunner(log *slog.Logger, manifest ManifestStore, objectReader ObjectReader, publisher ReplayPublisher) (*Runner, error) {
	if manifest == nil {
		return nil, fmt.Errorf("manifest store is required")
	}
	if objectReader == nil {
		return nil, fmt.Errorf("object reader is required")
	}
	if publisher == nil {
		return nil, fmt.Errorf("replay publisher is required")
	}
	if log == nil {
		log = slog.Default()
	}
	return &Runner{
		log:          log,
		manifest:     manifest,
		objectReader: objectReader,
		publisher:    publisher,
	}, nil
}

func (r *Runner) Close() error {
	var firstErr error
	if r.publisher != nil {
		if err := r.publisher.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.objectReader != nil {
		if err := r.objectReader.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	if r.manifest != nil {
		if err := r.manifest.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (r *Runner) ReplayDevices(ctx context.Context, request ReplayRequest) (ReplayReport, error) {
	if err := request.Validate("device"); err != nil {
		return ReplayReport{}, err
	}
	report := ReplayReport{
		Mode:             "device",
		PublishedByShard: make(map[uint32]int),
		StartedAt:        time.Now().UTC(),
	}
	deviceIDs := normalizeStrings(request.DeviceIDs, false)
	providerDeviceIDs := normalizeStrings(request.ProviderDeviceIDs, true)
	objects, err := r.manifest.ListByDevices(ctx, DeviceQuery{
		Provider:           strings.TrimSpace(request.Provider),
		FromUnixMS:         request.FromUnixMS,
		ToUnixMS:           request.ToUnixMS,
		DeviceIDs:          deviceIDs,
		ProviderDeviceIDs:  providerDeviceIDs,
		MaxObjectsReturned: request.MaxObjects,
	})
	if err != nil {
		return report, fmt.Errorf("query manifests by devices: %w", err)
	}
	report.ObjectsMatched = len(objects)
	deviceFilter := makeSet(deviceIDs)
	providerFilter := makeSet(providerDeviceIDs)
	err = r.processObjects(ctx, objects, report.Mode, func(env *envelopev1.TelemetryEnvelope) bool {
		if env == nil {
			return false
		}
		if _, ok := deviceFilter[strings.TrimSpace(env.GetDeviceId())]; ok {
			return true
		}
		providerDeviceID := strings.ToUpper(strings.TrimSpace(env.GetEcoflowSn()))
		if labels := env.GetLabels(); len(labels) > 0 {
			if candidate := strings.ToUpper(strings.TrimSpace(labels["provider_device_id"])); candidate != "" {
				providerDeviceID = candidate
			}
		}
		_, ok := providerFilter[providerDeviceID]
		return ok
	}, &report)
	report.FinishedAt = time.Now().UTC()
	if err != nil {
		return report, err
	}
	return report, nil
}

func (r *Runner) ReplayFleet(ctx context.Context, request ReplayRequest) (ReplayReport, error) {
	if err := request.Validate("fleet"); err != nil {
		return ReplayReport{}, err
	}
	report := ReplayReport{
		Mode:             "fleet",
		PublishedByShard: make(map[uint32]int),
		StartedAt:        time.Now().UTC(),
	}
	objects, err := r.manifest.ListByFleetRange(ctx, FleetQuery{
		FromUnixMS:         request.FromUnixMS,
		ToUnixMS:           request.ToUnixMS,
		Shards:             request.Shards,
		MaxObjectsReturned: request.MaxObjects,
	})
	if err != nil {
		return report, fmt.Errorf("query manifests by fleet window: %w", err)
	}
	report.ObjectsMatched = len(objects)
	err = r.processObjects(ctx, objects, report.Mode, func(_ *envelopev1.TelemetryEnvelope) bool { return true }, &report)
	report.FinishedAt = time.Now().UTC()
	if err != nil {
		return report, err
	}
	return report, nil
}

func (r *Runner) processObjects(
	ctx context.Context,
	objects []ManifestObject,
	mode string,
	shouldPublish func(*envelopev1.TelemetryEnvelope) bool,
	report *ReplayReport,
) error {
	for _, object := range objects {
		body, err := r.objectReader.ReadObject(ctx, object.ObjectBucket, object.ObjectKey)
		if err != nil {
			report.MessagesFailed++
			return fmt.Errorf("read archive object %s/%s: %w", object.ObjectBucket, object.ObjectKey, err)
		}
		report.ObjectsProcessed++
		frames, err := DecodeEnvelopeFrames(body)
		if err != nil {
			report.MessagesFailed++
			return fmt.Errorf("decode archive object %s/%s: %w", object.ObjectBucket, object.ObjectKey, err)
		}
		for _, frame := range frames {
			if err := ctx.Err(); err != nil {
				return err
			}
			report.MessagesDecoded++
			var env envelopev1.TelemetryEnvelope
			if err := proto.Unmarshal(frame, &env); err != nil {
				report.MessagesFailed++
				return fmt.Errorf("unmarshal telemetry envelope from %s/%s: %w", object.ObjectBucket, object.ObjectKey, err)
			}
			ts := envelopeTimestampUnixMS(&env)
			if ts > 0 {
				if report.FirstMessageUnixMS == 0 || ts < report.FirstMessageUnixMS {
					report.FirstMessageUnixMS = ts
				}
				if ts > report.LastMessageUnixMS {
					report.LastMessageUnixMS = ts
				}
			}
			if !shouldPublish(&env) {
				report.MessagesFiltered++
				continue
			}
			shard := normalizeEnvelopeShard(&env, object.Shard)
			if err := r.publisher.Publish(ctx, shard, frame); err != nil {
				report.MessagesFailed++
				return fmt.Errorf("publish replay envelope mode=%s shard=%d: %w", mode, shard, err)
			}
			report.MessagesPublished++
			report.PublishedByShard[shard]++
		}
	}
	return nil
}

func normalizeEnvelopeShard(env *envelopev1.TelemetryEnvelope, fallback uint32) uint32 {
	if env == nil {
		return fallback
	}
	if env.GetShardCount() == 0 {
		return fallback
	}
	shard := env.GetShard()
	if shard >= env.GetShardCount() {
		return fallback
	}
	return shard
}

func envelopeTimestampUnixMS(env *envelopev1.TelemetryEnvelope) int64 {
	if env == nil {
		return 0
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
	return 0
}

func makeSet(values []string) map[string]struct{} {
	if len(values) == 0 {
		return map[string]struct{}{}
	}
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		out[value] = struct{}{}
	}
	return out
}

type NoopPublisher struct{}

func (NoopPublisher) Publish(_ context.Context, _ uint32, _ []byte) error { return nil }
func (NoopPublisher) Close() error                                        { return nil }
