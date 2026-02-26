package main

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strconv"
	"strings"

	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
)

type deviceQuotaResult struct {
	quota map[string]string
	err   error
}

type batterySOCEntry struct {
	source    string
	label     string
	batterySN string
	soc       float64
}

type pvInputEntry struct {
	key   string
	watts float64
}

func main() {
	client, err := ecoflow.NewClientFromEnvironment()
	if err != nil {
		log.Fatalf("client init failed: %v", err)
	}
	ctx := context.Background()

	devices, response, err := client.GeneralInfo().ListDevices(ctx)
	if err != nil {
		log.Printf("request failed: %v", err)
	}

	fmt.Printf("status=%d devices=%d\n", response.StatusCode, len(devices))
	for i, device := range devices {
		fmt.Printf(
			"device[%d]: sn=%s deviceName=%q productName=%q online=%d\n",
			i,
			device.SN,
			device.DeviceName,
			device.ProductName,
			device.Online,
		)
	}

	certification, certificationResponse, err := client.GeneralInfo().GetMQTTCertification(ctx)
	if err != nil {
		log.Printf("mqtt certification request failed: %v", err)
	}
	fmt.Printf(
		"mqtt_cert_status=%d protocol=%s host=%s port=%s account=%s password=%s\n",
		certificationResponse.StatusCode,
		certification.Protocol,
		certification.URL,
		certification.Port,
		certification.CertificateAccount,
		maskSecret(certification.CertificatePassword),
	)

	quotaResults := make(map[string]deviceQuotaResult, len(devices))
	for i, device := range devices {
		quota, quotaResponse, quotaErr := client.GeneralInfo().GetDeviceAllQuota(ctx, device.SN)
		if quotaErr != nil {
			quotaResults[device.SN] = deviceQuotaResult{err: quotaErr}
			if ecoflow.IsBusinessErrorCode(quotaErr, "1006") {
				log.Printf(
					"device all quota skipped (device[%d] sn=%s): access denied by EcoFlow (code=1006)",
					i,
					device.SN,
				)
				continue
			}
			log.Printf(
				"device all quota request failed (device[%d] sn=%s): %v",
				i,
				device.SN,
				quotaErr,
			)
			continue
		}
		quotaResults[device.SN] = deviceQuotaResult{quota: quota}
		fmt.Printf(
			"quota_status=%d sn=%s deviceName=%q quota_count=%d\n",
			quotaResponse.StatusCode,
			device.SN,
			device.DeviceName,
			len(quota),
		)
		// printQuotaAll(quota)
		// printTypedBPInfo(quota)
		// printTypedKitInfoWatts(quota)
		// printTypedUnsignedIntArrays(quota)
	}

	printBatteryAndPVSummary(devices, quotaResults)
}

func maskSecret(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + "****" + value[len(value)-2:]
}

//nolint:unused // local debug helper for full quota dumps.
func printQuotaAll(quota map[string]string) {
	if len(quota) == 0 {
		return
	}
	keys := make([]string, 0, len(quota))
	for key := range quota {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Printf("quota[%s]=%s\n", key, quota[key])
	}
}

//nolint:unused // local debug helper for typed battery-pack fields.
func printTypedBPInfo(quota map[string]string) {
	if len(quota) == 0 {
		return
	}
	keys := make([]string, 0, len(quota))
	for key := range quota {
		if strings.HasSuffix(key, ".bpInfo") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		packs, found, err := ecoflow.ParseQuotaBatteryPackInfo(quota, key)
		if err != nil {
			fmt.Printf("bpInfo_parse_error key=%s err=%v\n", key, err)
			continue
		}
		if !found || len(packs) == 0 {
			continue
		}
		for _, pack := range packs {
			fmt.Printf(
				"bpInfo[%s]: bpNo=%d soc=%d temp=%d pwr=%d remainTime=%d chgSta=%d errCode=%d\n",
				key,
				pack.BPNo,
				pack.BPSoc,
				pack.BPTemp,
				pack.BPPwr,
				pack.RemainTime,
				pack.BPChgSta,
				pack.BPErrCode,
			)
		}
	}
}

//nolint:unused // local debug helper for typed kitInfo watts arrays.
func printTypedKitInfoWatts(quota map[string]string) {
	if len(quota) == 0 {
		return
	}
	keys := make([]string, 0, len(quota))
	for key := range quota {
		if strings.HasSuffix(key, "kitInfo.watts") {
			keys = append(keys, key)
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		entries, found, err := ecoflow.ParseQuotaKitInfoWatts(quota, key)
		if err != nil {
			fmt.Printf("kitInfoWatts_parse_error key=%s err=%v\n", key, err)
			continue
		}
		if !found || len(entries) == 0 {
			continue
		}
		for i, entry := range entries {
			fmt.Printf(
				"kitInfoWatts[%s][%d]: sn=%q soc=%d f32Soc=%.2f curPower=%d appState=%d avaFlag=%d type=%d detail=%d\n",
				key,
				i,
				entry.SN,
				entry.Soc,
				entry.F32Soc,
				entry.CurPower,
				entry.AppState,
				entry.AvaFlag,
				entry.Type,
				entry.Detail,
			)
		}
	}
}

//nolint:unused // local debug helper for typed uint-array quota values.
func printTypedUnsignedIntArrays(quota map[string]string) {
	if len(quota) == 0 {
		return
	}
	targetSuffixes := []string{
		".cellTemp",
		".cellVol",
		".allErrFlag",
		".mosTemp",
		".ptcTemp",
		".icoBytes",
		".reserved",
		".hwVersion",
		".bmsIsConnt",
		".bmsKitState",
	}

	keys := make([]string, 0, len(quota))
	for key := range quota {
		for _, suffix := range targetSuffixes {
			if strings.HasSuffix(key, suffix) {
				keys = append(keys, key)
				break
			}
		}
	}
	sort.Strings(keys)
	for _, key := range keys {
		values, found, err := ecoflow.ParseQuotaUnsignedIntArray(quota, key)
		if err != nil || !found {
			continue
		}
		fmt.Printf("uintArray[%s]=%v\n", key, values)
	}
}

func printBatteryAndPVSummary(devices []ecoflow.GeneralInfoDevice, results map[string]deviceQuotaResult) {
	fmt.Println("=== Battery + PV Summary ===")
	for _, device := range devices {
		result, ok := results[device.SN]
		if !ok {
			fmt.Printf("energy_summary sn=%s deviceName=%q status=missing\n", device.SN, device.DeviceName)
			continue
		}
		if result.err != nil {
			if ecoflow.IsBusinessErrorCode(result.err, "1006") {
				fmt.Printf("energy_summary sn=%s deviceName=%q status=skipped reason=access_denied\n", device.SN, device.DeviceName)
			} else {
				fmt.Printf("energy_summary sn=%s deviceName=%q status=error err=%q\n", device.SN, device.DeviceName, result.err.Error())
			}
			continue
		}

		batteryEntries := extractBatterySOCEntries(result.quota)
		pvEntries := extractPVInputEntries(result.quota)
		fmt.Printf(
			"energy_summary sn=%s deviceName=%q status=ok batteries=%d pv_inputs=%d\n",
			device.SN,
			device.DeviceName,
			len(batteryEntries),
			len(pvEntries),
		)
		for i, entry := range batteryEntries {
			fmt.Printf(
				"battery_soc[%d] source=%s label=%q battery_sn=%q soc=%.2f\n",
				i,
				entry.source,
				entry.label,
				entry.batterySN,
				entry.soc,
			)
		}
		for i, entry := range pvEntries {
			fmt.Printf("pv_input[%d] key=%q watts=%.3f\n", i, entry.key, entry.watts)
		}
	}
}

func extractBatterySOCEntries(quota map[string]string) []batterySOCEntry {
	keys := sortedKeys(quota)
	out := make([]batterySOCEntry, 0, 8)

	for _, key := range keys {
		if !strings.HasSuffix(key, ".bpInfo") {
			continue
		}
		packs, found, err := ecoflow.ParseQuotaBatteryPackInfo(quota, key)
		if err != nil || !found {
			continue
		}
		for _, pack := range packs {
			out = append(out, batterySOCEntry{
				source: "bpInfo",
				label:  fmt.Sprintf("%s.bpNo=%d", key, pack.BPNo),
				soc:    float64(pack.BPSoc),
			})
		}
	}

	for _, key := range keys {
		if !strings.HasSuffix(strings.ToLower(key), "kitinfo.watts") {
			continue
		}
		entries, found, err := ecoflow.ParseQuotaKitInfoWatts(quota, key)
		if err != nil || !found {
			continue
		}
		for i, entry := range entries {
			if entry.AvaFlag == 0 && entry.SN == "" && entry.Soc == 0 {
				continue
			}
			out = append(out, batterySOCEntry{
				source:    "kitInfo.watts",
				label:     fmt.Sprintf("%s[%d]", key, i),
				batterySN: entry.SN,
				soc:       float64(entry.Soc),
			})
		}
	}

	for _, key := range keys {
		lower := strings.ToLower(key)
		if !strings.HasSuffix(lower, ".soc") {
			continue
		}
		if !strings.Contains(lower, "bms_slave") && !strings.Contains(lower, "bmsstatus") && !strings.Contains(lower, "bms_slave_addr") {
			continue
		}
		soc, err := strconv.ParseFloat(strings.TrimSpace(quota[key]), 64)
		if err != nil {
			continue
		}
		out = append(out, batterySOCEntry{
			source: "quota.soc",
			label:  key,
			soc:    soc,
		})
	}

	return out
}

func extractPVInputEntries(quota map[string]string) []pvInputEntry {
	keys := sortedKeys(quota)
	out := make([]pvInputEntry, 0, 6)
	for _, key := range keys {
		if !isPVInputKey(key) {
			continue
		}
		watts, err := strconv.ParseFloat(strings.TrimSpace(quota[key]), 64)
		if err != nil {
			continue
		}
		out = append(out, pvInputEntry{key: key, watts: watts})
	}
	return out
}

func isPVInputKey(key string) bool {
	lower := strings.ToLower(key)
	if strings.Contains(lower, "mppt.inwatts") {
		return true
	}
	if strings.Contains(lower, "inhvmpptpwr") || strings.Contains(lower, "inlvmpptpwr") {
		return true
	}
	if strings.Contains(lower, "pv") {
		if strings.Contains(lower, "chargewatts") || strings.Contains(lower, "inwatts") || strings.HasSuffix(lower, "pwr") {
			return true
		}
	}
	return false
}

func sortedKeys(values map[string]string) []string {
	out := make([]string, 0, len(values))
	for key := range values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}
