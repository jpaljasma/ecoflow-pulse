package rolluprebuild

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParseRawLogLine(t *testing.T) {
	t.Parallel()

	line := `2026-03-07T08:24:35.15003-05:00 topic=/open/account/DEMODPU0000294/quota payload_raw={"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"inLvMpptPwr":54.476166}}`
	at, topic, payload, err := parseRawLogLine(line)
	if err != nil {
		t.Fatalf("parseRawLogLine failed: %v", err)
	}
	if want := time.Date(2026, time.March, 7, 13, 24, 35, 150030000, time.UTC); !at.Equal(want) {
		t.Fatalf("timestamp mismatch: got=%s want=%s", at, want)
	}
	if topic != "/open/account/DEMODPU0000294/quota" {
		t.Fatalf("topic mismatch: got=%q", topic)
	}
	if string(payload) != `{"cmdId":1,"cmdFunc":2,"addr":"hs_yj751_pd_appshow_addr","params":{"inLvMpptPwr":54.476166}}` {
		t.Fatalf("payload mismatch: got=%s", payload)
	}
}

func TestCollectRawLogEventsFeedsAuthoritativeSolarAggregation(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "mqtt_payload_raw.log")
	content := "" +
		"2026-03-06T12:00:30Z topic=/open/account/DEMODPU0000294/quota payload_raw={\"params\":{\"inLvMpptPwr\":120}}\n" +
		"2026-03-06T12:01:15Z topic=/open/account/DEMODPU0000294/quota payload_raw={\"params\":{\"inLvMpptPwr\":60}}\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write temp raw log: %v", err)
	}

	report := &Report{}
	events, affected, err := collectRawLogEvents(
		context.Background(),
		"ecoflow",
		[]string{path},
		time.Date(2026, time.March, 6, 12, 2, 0, 0, time.UTC),
		map[string]string{"DEMODPU0000294": "018f23f1-3b3d-7f27-b2fd-6f6f68ef5f52"},
		report,
	)
	if err != nil {
		t.Fatalf("collectRawLogEvents failed: %v", err)
	}
	if got := len(affected); got != 1 {
		t.Fatalf("affected device count mismatch: got=%d want=1", got)
	}

	agg := NewAggregator()
	for _, event := range events {
		agg.ApplySample(event.sample)
	}
	agg.Finalize(time.Date(2026, time.March, 6, 12, 2, 0, 0, time.UTC))
	rows := agg.Rows(ResolutionMinute)
	if len(rows) != 2 {
		t.Fatalf("minute row count mismatch: got=%d want=2", len(rows))
	}
	if rows[0].SolarGeneratedWh != 1 {
		t.Fatalf("first minute solar mismatch: got=%f want=1", rows[0].SolarGeneratedWh)
	}
	if rows[1].SolarGeneratedWh != 1.25 {
		t.Fatalf("second minute solar mismatch: got=%f want=1.25", rows[1].SolarGeneratedWh)
	}
	if report.MessagesDecoded != 2 {
		t.Fatalf("decoded message count mismatch: got=%d want=2", report.MessagesDecoded)
	}
}
