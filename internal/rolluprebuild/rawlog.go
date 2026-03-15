package rolluprebuild

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	envelopev1 "github.com/jpaljasma/ecoflow-pulse/gen/pulse/envelope/v1"
	"github.com/jpaljasma/ecoflow-pulse/internal/rollupworker"
)

type rawLogEvent struct {
	sample *rollupworker.RollupSample
	seq    int
}

func RebuildFromRawLogs(
	ctx context.Context,
	writer *PostgresWriter,
	provider string,
	from time.Time,
	to time.Time,
	rawLogInputs []string,
	deviceIDs []string,
	providerDeviceIDs []string,
	chunkSize int,
) (Report, error) {
	report := Report{StartedAt: time.Now().UTC()}
	if writer == nil {
		return report, fmt.Errorf("postgres writer is required")
	}
	if !to.After(from) {
		return report, fmt.Errorf("raw log rebuild window must have from < to")
	}
	paths, err := expandRawLogPaths(rawLogInputs)
	if err != nil {
		return report, err
	}
	report.ObjectsMatched = len(paths)
	if len(paths) == 0 {
		report.FinishedAt = time.Now().UTC()
		return report, nil
	}

	provider = strings.TrimSpace(provider)
	if provider == "" {
		provider = "ecoflow"
	}

	if len(deviceIDs) == 0 && len(providerDeviceIDs) == 0 {
		providerDeviceIDs, err = collectProviderDeviceIDs(paths)
		if err != nil {
			return report, err
		}
	}
	deviceMap, err := writer.ResolveDeviceMappings(ctx, provider, deviceIDs, providerDeviceIDs)
	if err != nil {
		return report, err
	}
	if len(deviceMap) == 0 {
		return report, fmt.Errorf("no provider device mappings found for raw log rebuild")
	}

	events, affected, err := collectRawLogEvents(ctx, provider, paths, to.UTC(), deviceMap, &report)
	if err != nil {
		return report, err
	}
	sort.Slice(events, func(i, j int) bool {
		left := events[i].sample.EventTime
		right := events[j].sample.EventTime
		if !left.Equal(right) {
			return left.Before(right)
		}
		if events[i].sample.ProviderDeviceID != events[j].sample.ProviderDeviceID {
			return events[i].sample.ProviderDeviceID < events[j].sample.ProviderDeviceID
		}
		return events[i].seq < events[j].seq
	})

	aggregator := NewAggregator()
	for _, event := range events {
		aggregator.ApplySample(event.sample)
		report.MessagesApplied++
	}
	aggregator.Finalize(to.UTC())

	minuteRowsAll := filterRowsForWindow(aggregator.Rows(ResolutionMinute), ResolutionMinute, from.UTC(), to.UTC())
	hourRowsAll := filterRowsForWindow(aggregator.Rows(ResolutionHour), ResolutionHour, from.UTC(), to.UTC())
	dayRowsAll := filterRowsForWindow(aggregator.Rows(ResolutionDay), ResolutionDay, from.UTC(), to.UTC())
	pvPortMinuteRowsAll := filterPVPortRowsForWindow(aggregator.PVPortRows(ResolutionMinute), ResolutionMinute, from.UTC(), to.UTC())
	pvPortHourRowsAll := filterPVPortRowsForWindow(aggregator.PVPortRows(ResolutionHour), ResolutionHour, from.UTC(), to.UTC())
	pvPortDayRowsAll := filterPVPortRowsForWindow(aggregator.PVPortRows(ResolutionDay), ResolutionDay, from.UTC(), to.UTC())
	if report.MinuteRows, err = writer.ReplaceRows(ctx, ResolutionMinute, minuteRowsAll, affected, from.UTC(), to.UTC(), chunkSize); err != nil {
		return report, err
	}
	if report.HourRows, err = writer.ReplaceRows(ctx, ResolutionHour, hourRowsAll, affected, from.UTC(), to.UTC(), chunkSize); err != nil {
		return report, err
	}
	if report.DayRows, err = writer.ReplaceRows(ctx, ResolutionDay, dayRowsAll, affected, from.UTC(), to.UTC(), chunkSize); err != nil {
		return report, err
	}
	if report.PVPortMinuteRows, err = writer.ReplacePVPortRows(ctx, ResolutionMinute, pvPortMinuteRowsAll, affected, from.UTC(), to.UTC(), chunkSize); err != nil {
		return report, err
	}
	if report.PVPortHourRows, err = writer.ReplacePVPortRows(ctx, ResolutionHour, pvPortHourRowsAll, affected, from.UTC(), to.UTC(), chunkSize); err != nil {
		return report, err
	}
	if report.PVPortDayRows, err = writer.ReplacePVPortRows(ctx, ResolutionDay, pvPortDayRowsAll, affected, from.UTC(), to.UTC(), chunkSize); err != nil {
		return report, err
	}
	report.FinishedAt = time.Now().UTC()
	return report, nil
}

func collectRawLogEvents(
	ctx context.Context,
	provider string,
	paths []string,
	to time.Time,
	deviceMap map[string]string,
	report *Report,
) ([]rawLogEvent, []DeviceWindow, error) {
	events := make([]rawLogEvent, 0, 1024)
	affected := make([]DeviceWindow, 0, len(deviceMap))
	for providerDeviceID := range deviceMap {
		affected = append(affected, DeviceWindow{
			Provider:         provider,
			ProviderDeviceID: providerDeviceID,
		})
	}
	seq := 0
	for _, path := range paths {
		if err := ctx.Err(); err != nil {
			return nil, nil, err
		}
		file, err := os.Open(path)
		if err != nil {
			return nil, nil, fmt.Errorf("open raw log %s: %w", path, err)
		}
		report.ObjectsProcessed++
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		lineNo := 0
		for scanner.Scan() {
			lineNo++
			if err := ctx.Err(); err != nil {
				_ = file.Close()
				return nil, nil, err
			}
			at, topic, payload, err := parseRawLogLine(scanner.Text())
			if err != nil {
				continue
			}
			if !at.Before(to) {
				continue
			}
			providerDeviceID := providerDeviceIDFromTopic(topic)
			deviceID, ok := deviceMap[providerDeviceID]
			if !ok {
				continue
			}
			report.MessagesDecoded++
			env := &envelopev1.TelemetryEnvelope{
				DeviceId:           deviceID,
				EcoflowSn:          providerDeviceID,
				ObservedTimeUnixMs: at.UnixMilli(),
				Payload:            payload,
				Labels:             map[string]string{"provider": provider},
			}
			sample, sampleErr := rollupworker.SampleFromEnvelope(env)
			if sampleErr == rollupworker.ErrNoRollupMetrics {
				continue
			}
			if sampleErr != nil {
				_ = file.Close()
				return nil, nil, fmt.Errorf("derive rollup sample from raw log %s:%d: %w", path, lineNo, sampleErr)
			}
			events = append(events, rawLogEvent{sample: sample, seq: seq})
			seq++
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return nil, nil, fmt.Errorf("scan raw log %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return nil, nil, fmt.Errorf("close raw log %s: %w", path, err)
		}
	}
	return events, normalizeAffectedDevices(affected), nil
}

func expandRawLogPaths(inputs []string) ([]string, error) {
	if len(inputs) == 0 {
		return nil, fmt.Errorf("at least one raw log path is required")
	}
	seen := make(map[string]struct{})
	paths := make([]string, 0, len(inputs))
	for _, input := range inputs {
		input = strings.TrimSpace(input)
		if input == "" {
			continue
		}
		matches := []string{input}
		if hasGlobMeta(input) {
			var err error
			matches, err = filepath.Glob(input)
			if err != nil {
				return nil, fmt.Errorf("expand raw log glob %s: %w", input, err)
			}
		}
		for _, match := range matches {
			match = filepath.Clean(match)
			if _, exists := seen[match]; exists {
				continue
			}
			seen[match] = struct{}{}
			paths = append(paths, match)
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func collectProviderDeviceIDs(paths []string) ([]string, error) {
	seen := make(map[string]struct{})
	out := make([]string, 0, 8)
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			return nil, fmt.Errorf("open raw log %s: %w", path, err)
		}
		scanner := bufio.NewScanner(file)
		scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
		for scanner.Scan() {
			_, topic, _, err := parseRawLogLine(scanner.Text())
			if err != nil {
				continue
			}
			providerDeviceID := providerDeviceIDFromTopic(topic)
			if providerDeviceID == "" {
				continue
			}
			if _, exists := seen[providerDeviceID]; exists {
				continue
			}
			seen[providerDeviceID] = struct{}{}
			out = append(out, providerDeviceID)
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			return nil, fmt.Errorf("scan raw log %s: %w", path, err)
		}
		if err := file.Close(); err != nil {
			return nil, fmt.Errorf("close raw log %s: %w", path, err)
		}
	}
	sort.Strings(out)
	return out, nil
}

func parseRawLogLine(line string) (time.Time, string, []byte, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return time.Time{}, "", nil, fmt.Errorf("empty raw log line")
	}
	topicIdx := strings.Index(line, " topic=")
	payloadIdx := strings.Index(line, " payload_raw=")
	if topicIdx <= 0 || payloadIdx <= topicIdx {
		return time.Time{}, "", nil, fmt.Errorf("raw log line missing topic/payload")
	}
	at, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(line[:topicIdx]))
	if err != nil {
		return time.Time{}, "", nil, fmt.Errorf("parse raw log timestamp: %w", err)
	}
	topic := strings.TrimSpace(line[topicIdx+7 : payloadIdx])
	payload := []byte(strings.TrimSpace(line[payloadIdx+13:]))
	if topic == "" || len(payload) == 0 {
		return time.Time{}, "", nil, fmt.Errorf("raw log line missing topic/payload body")
	}
	return at.UTC(), topic, payload, nil
}

func providerDeviceIDFromTopic(topic string) string {
	topic = strings.TrimSpace(topic)
	if topic == "" {
		return ""
	}
	parts := strings.Split(strings.Trim(topic, "/"), "/")
	if len(parts) < 2 {
		return ""
	}
	if !strings.EqualFold(parts[len(parts)-1], "quota") {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(parts[len(parts)-2]))
}

func hasGlobMeta(path string) bool {
	return strings.ContainsAny(path, "*?[")
}
