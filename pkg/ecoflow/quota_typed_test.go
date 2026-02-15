package ecoflow

import (
	"strings"
	"testing"
)

func TestParseBatteryPackInfo(t *testing.T) {
	t.Parallel()

	input := `[{"bpChgSta":0,"bpEnergy":6144,"bpErrCode":0,"bpNo":1,"bpPwr":0,"bpSoc":17,"bpSocMax":95,"bpSocMin":5,"bpSunnovaBan":0,"bpTemp":8,"heatTime":0,"remainTime":4185},{"bpChgSta":0,"bpEnergy":6144,"bpErrCode":0,"bpNo":2,"bpPwr":0,"bpSoc":24,"bpSocMax":95,"bpSocMin":5,"bpSunnovaBan":0,"bpTemp":7,"heatTime":0,"remainTime":5704}]`

	packs, err := ParseBatteryPackInfo(input)
	if err != nil {
		t.Fatalf("ParseBatteryPackInfo() error = %v", err)
	}
	if len(packs) != 2 {
		t.Fatalf("pack count mismatch: got %d", len(packs))
	}
	if packs[0].BPNo != 1 || packs[1].BPNo != 2 {
		t.Fatalf("pack number mismatch: got %+v", packs)
	}
	if packs[0].BPSoc != 17 || packs[1].BPSoc != 24 {
		t.Fatalf("pack soc mismatch: got %+v", packs)
	}
}

func TestParseQuotaBatteryPackInfo(t *testing.T) {
	t.Parallel()

	quota := map[string]string{
		"hs_yj751_pd_bp_addr.bpInfo": `[{"bpNo":1,"bpSoc":50}]`,
	}
	packs, found, err := ParseQuotaBatteryPackInfo(quota, "hs_yj751_pd_bp_addr.bpInfo")
	if err != nil {
		t.Fatalf("ParseQuotaBatteryPackInfo() error = %v", err)
	}
	if !found {
		t.Fatal("expected quota key found")
	}
	if len(packs) != 1 || packs[0].BPNo != 1 || packs[0].BPSoc != 50 {
		t.Fatalf("parsed packs mismatch: %+v", packs)
	}
}

func TestParseQuotaBatteryPackInfo_InvalidJSON(t *testing.T) {
	t.Parallel()

	quota := map[string]string{
		"k.bpInfo": `{"invalid":"shape"}`,
	}
	_, found, err := ParseQuotaBatteryPackInfo(quota, "k.bpInfo")
	if !found {
		t.Fatal("expected quota key found")
	}
	if err == nil {
		t.Fatal("expected parse error, got nil")
	}
	if !strings.Contains(err.Error(), "parse bpInfo") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseKitInfoWatts(t *testing.T) {
	t.Parallel()

	input := `[{"appState":0,"appVer":0,"avaFlag":0,"curPower":0,"detail":0,"f32Soc":0,"loadVer":0,"sn":"","soc":0,"type":0},{"appState":1,"appVer":33620275,"avaFlag":1,"curPower":-77,"detail":4,"f32Soc":47.97,"loadVer":33619974,"sn":"R361Z1BAPH2K1398","soc":48,"type":81}]`

	entries, err := ParseKitInfoWatts(input)
	if err != nil {
		t.Fatalf("ParseKitInfoWatts() error = %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("entry count mismatch: got %d", len(entries))
	}
	if entries[0].Soc != 0 || entries[1].Soc != 48 {
		t.Fatalf("soc mismatch: got %+v", entries)
	}
	if entries[1].CurPower != -77 {
		t.Fatalf("curPower mismatch: got %d", entries[1].CurPower)
	}
	if entries[1].SN != "R361Z1BAPH2K1398" {
		t.Fatalf("sn mismatch: got %q", entries[1].SN)
	}
}

func TestParseQuotaKitInfoWatts(t *testing.T) {
	t.Parallel()

	quota := map[string]string{
		"bms_kitInfo.watts": `[{"appState":1,"soc":50}]`,
	}
	entries, found, err := ParseQuotaKitInfoWatts(quota, "bms_kitInfo.watts")
	if err != nil {
		t.Fatalf("ParseQuotaKitInfoWatts() error = %v", err)
	}
	if !found {
		t.Fatal("expected quota key found")
	}
	if len(entries) != 1 || entries[0].AppState != 1 || entries[0].Soc != 50 {
		t.Fatalf("parsed entries mismatch: %+v", entries)
	}
}

func TestParseUnsignedIntArray(t *testing.T) {
	t.Parallel()

	values, err := ParseUnsignedIntArray("[0,9,11,17,11,15,8,8]")
	if err != nil {
		t.Fatalf("ParseUnsignedIntArray() error = %v", err)
	}
	if len(values) != 8 {
		t.Fatalf("array length mismatch: got %d", len(values))
	}
	if values[3] != 17 || values[7] != 8 {
		t.Fatalf("array values mismatch: got %v", values)
	}
}

func TestParseQuotaUnsignedIntArray(t *testing.T) {
	t.Parallel()

	quota := map[string]string{
		"k.cellTemp": "[7,9,12,9]",
	}
	values, found, err := ParseQuotaUnsignedIntArray(quota, "k.cellTemp")
	if err != nil {
		t.Fatalf("ParseQuotaUnsignedIntArray() error = %v", err)
	}
	if !found {
		t.Fatal("expected key found")
	}
	if len(values) != 4 || values[2] != 12 {
		t.Fatalf("parsed array mismatch: %v", values)
	}
}

func TestParseUnsignedIntArray_InvalidNegativeFails(t *testing.T) {
	t.Parallel()

	_, err := ParseUnsignedIntArray("[-1,2]")
	if err == nil {
		t.Fatal("expected parse error for negative value, got nil")
	}
	if !strings.Contains(err.Error(), "non-uint value") {
		t.Fatalf("unexpected error: %v", err)
	}
}
