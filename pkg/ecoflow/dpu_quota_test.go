package ecoflow

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestParseDPUQuota(t *testing.T) {
	t.Parallel()

	values := map[string]string{
		"hs_yj751_pd_appshow_addr.outAcTtPwr":     "119",
		"hs_yj751_pd_appshow_addr.soc":            "21",
		"hs_yj751_pd_appshow_addr.remainTime":     "4598",
		"hs_yj751_pd_app_set_info_addr.dsgMinSoc": "5",
		"hs_yj751_pd_app_set_info_addr.chgMaxSoc": "95",
		"hs_yj751_pd_backend_addr.batVol":         "104.277",
		"hs_yj751_pd_backend_addr.batAmp":         "0.235",
		"hs_yj751_pd_bp_addr.bpInfo":              `[{"bpNo":1,"bpSoc":17,"bpTemp":8}]`,
		"hs_yj751_pd_bp_addr.updateTime":          "2026-02-14 23:40:44",
		"hs_yj751_bms_slave_addr.1.packSn":        "Y716ZA1BBHBN0478",
		"hs_yj751_bms_slave_addr.1.soc":           "17",
		"hs_yj751_bms_slave_addr.1.cellTemp":      "[9,11,17,11,15,8,8]",
		"hs_yj751_bms_slave_addr.1.cellVol":       "[3260,3260,3261]",
		"hs_yj751_bms_slave_addr.1.allErrFlag":    "[0,0]",
	}

	quota, err := ParseDPUQuota(values)
	if err != nil {
		t.Fatalf("ParseDPUQuota() error = %v", err)
	}
	if quota.AppShow.OutAcTtPwr != 119 {
		t.Fatalf("OutAcTtPwr mismatch: got %v", quota.AppShow.OutAcTtPwr)
	}
	if quota.AppShow.Soc != 21 {
		t.Fatalf("AppShow.Soc mismatch: got %d", quota.AppShow.Soc)
	}
	if quota.AppSet.DsgMinSoc != 5 || quota.AppSet.ChgMaxSoc != 95 {
		t.Fatalf("AppSet fields mismatch: %+v", quota.AppSet)
	}
	if quota.Backend.BatVol != 104.277 || quota.Backend.BatAmp != 0.235 {
		t.Fatalf("Backend fields mismatch: %+v", quota.Backend)
	}
	if len(quota.BatteryPackInfo) != 1 || quota.BatteryPackInfo[0].BPNo != 1 {
		t.Fatalf("BatteryPackInfo mismatch: %+v", quota.BatteryPackInfo)
	}
	if quota.BatteryPackInfoUpdatedAt != "2026-02-14 23:40:44" {
		t.Fatalf("BatteryPackInfoUpdatedAt mismatch: got %q", quota.BatteryPackInfoUpdatedAt)
	}
	slave, ok := quota.BMSSlaves[1]
	if !ok {
		t.Fatal("expected BMS slave index 1")
	}
	if slave.PackSN != "Y716ZA1BBHBN0478" || slave.Soc != 17 {
		t.Fatalf("BMS slave scalar mismatch: %+v", slave)
	}
	if len(slave.CellTemp) != 7 || slave.CellTemp[2] != 17 {
		t.Fatalf("BMS slave cellTemp mismatch: %v", slave.CellTemp)
	}
	if len(slave.AllErrFlag) != 2 || slave.AllErrFlag[0] != 0 {
		t.Fatalf("BMS slave allErrFlag mismatch: %v", slave.AllErrFlag)
	}
}

func TestGeneralInfoService_GetDPUQuota(t *testing.T) {
	t.Parallel()

	credentialsProvider, err := NewStaticCredentialsProvider("ak-test-1234", "sk-test-1234")
	if err != nil {
		t.Fatalf("NewStaticCredentialsProvider() error = %v", err)
	}

	cfg := DefaultConfig()
	cfg.BaseURL = "https://api.ecoflow.test"
	cfg.CredentialsProvider = credentialsProvider
	cfg.HTTPClient = &http.Client{
		Transport: roundTripperFunc(func(req *http.Request) (*http.Response, error) {
			if req.Method != http.MethodGet {
				t.Fatalf("request method mismatch: got %q", req.Method)
			}
			if req.URL.Path != deviceQuotaAllPath {
				t.Fatalf("request path mismatch: got %q", req.URL.Path)
			}
			if got := req.URL.Query().Get("sn"); got != "Y711ZABA9H2P0294" {
				t.Fatalf("sn query mismatch: got %q", got)
			}
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     make(http.Header),
				Body: io.NopCloser(strings.NewReader(`{
					"code":"0",
					"message":"Success",
					"data":{
						"hs_yj751_pd_appshow_addr.outAcTtPwr":"119",
						"hs_yj751_bms_slave_addr.1.cellTemp":[9,11,17,11,15,8,8]
					}
				}`)),
			}, nil
		}),
	}

	client, err := NewClient(cfg)
	if err != nil {
		t.Fatalf("NewClient() error = %v", err)
	}

	quota, response, err := client.GeneralInfo().GetDPUQuota(context.Background(), "Y711ZABA9H2P0294")
	if err != nil {
		t.Fatalf("GetDPUQuota() error = %v", err)
	}
	if response.StatusCode != http.StatusOK {
		t.Fatalf("status code mismatch: got %d", response.StatusCode)
	}
	if quota.AppShow.OutAcTtPwr != 119 {
		t.Fatalf("OutAcTtPwr mismatch: got %v", quota.AppShow.OutAcTtPwr)
	}
	slave := quota.BMSSlaves[1]
	if len(slave.CellTemp) != 7 || slave.CellTemp[0] != 9 {
		t.Fatalf("CellTemp mismatch: %v", slave.CellTemp)
	}
}
