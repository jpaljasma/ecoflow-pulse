package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os/signal"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jpaljasma/ecoflow-pulse/internal/provideradapter"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflow"
	"github.com/jpaljasma/ecoflow-pulse/pkg/ecoflowmqtt"
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

type mqttProbeTarget struct {
	serialNumber string
	displayName  string
	topic        string
	seen         bool
	seenAt       time.Time
	messageCount int
}

type smokeConfig struct {
	mqttEnabled     bool
	mqttStatusEvery time.Duration
}

func main() {
	cfg := parseFlags()

	client, err := ecoflow.NewClientFromEnvironment()
	if err != nil {
		log.Fatalf("client init failed: %v", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

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

	if cfg.mqttEnabled {
		if err := runMQTTProbe(ctx, certification, devices, cfg.mqttStatusEvery); err != nil {
			log.Printf("mqtt probe failed: %v", err)
		}
	}
}

func parseFlags() smokeConfig {
	var cfg smokeConfig
	flag.BoolVar(&cfg.mqttEnabled, "mqtt", true, "subscribe to MQTT topics for all discovered devices")
	flag.DurationVar(&cfg.mqttStatusEvery, "mqtt-status-every", 5*time.Second, "interval for awaiting MQTT status updates")
	flag.Parse()
	return cfg
}

func maskSecret(value string) string {
	if len(value) <= 4 {
		return "****"
	}
	return value[:2] + "****" + value[len(value)-2:]
}

func runMQTTProbe(
	ctx context.Context,
	cert ecoflow.GeneralInfoMQTTCertification,
	devices []ecoflow.GeneralInfoDevice,
	statusEvery time.Duration,
) error {
	address, targets, err := buildMQTTProbeTargets(cert, devices)
	if err != nil {
		return err
	}
	if len(targets) == 0 {
		log.Printf("mqtt probe skipped: no discovered devices produced probe topics")
		return nil
	}
	if statusEvery <= 0 {
		statusEvery = 5 * time.Second
	}

	clientID := buildSmokeMQTTClientID(targets)
	subscriber, err := ecoflowmqtt.NewSubscriber(ecoflowmqtt.Config{
		Address:        address,
		Username:       strings.TrimSpace(cert.CertificateAccount),
		Password:       strings.TrimSpace(cert.CertificatePassword),
		ClientID:       clientID,
		KeepAlive:      60 * time.Second,
		ConnectTimeout: 10 * time.Second,
		ReadTimeout:    30 * time.Second,
		WriteTimeout:   15 * time.Second,
	})
	if err != nil {
		return fmt.Errorf("init mqtt subscriber: %w", err)
	}
	defer func() {
		if closeErr := subscriber.Close(); closeErr != nil {
			log.Printf("mqtt subscriber close failed: %v", closeErr)
		}
	}()

	log.Printf(
		"mqtt probe starting: devices=%d host=%s client_id=%s",
		len(targets),
		address,
		clientID,
	)
	if err := subscriber.Connect(ctx); err != nil {
		return fmt.Errorf("connect mqtt subscriber: %w", err)
	}
	log.Printf("mqtt connected: host=%s", address)

	targetsByTopic := make(map[string]*mqttProbeTarget, len(targets))
	for i := range targets {
		targetsByTopic[targets[i].topic] = &targets[i]
		if err := subscriber.Subscribe(ctx, targets[i].topic, 0); err != nil {
			return fmt.Errorf("subscribe mqtt topic for %s: %w", targets[i].serialNumber, err)
		}
		log.Printf(
			"mqtt subscribed: sn=%s name=%q topic=%s",
			targets[i].serialNumber,
			targets[i].displayName,
			targets[i].topic,
		)
	}

	startedAt := time.Now()
	logMQTTProbeStatus("mqtt awaiting live data", targets, startedAt)
	return readMQTTProbeLoop(ctx, subscriber, targetsByTopic, targets, statusEvery, startedAt)
}

func buildMQTTProbeTargets(
	cert ecoflow.GeneralInfoMQTTCertification,
	devices []ecoflow.GeneralInfoDevice,
) (string, []mqttProbeTarget, error) {
	if len(devices) == 0 {
		return "", nil, nil
	}
	seen := make(map[string]struct{}, len(devices))
	targets := make([]mqttProbeTarget, 0, len(devices))
	address := ""
	for i := range devices {
		sn := strings.ToUpper(strings.TrimSpace(devices[i].SN))
		if sn == "" {
			continue
		}
		if _, exists := seen[sn]; exists {
			continue
		}
		seen[sn] = struct{}{}
		topicAddress, topic, err := provideradapter.BuildMQTTAddressAndTopic(cert, sn)
		if err != nil {
			return "", nil, fmt.Errorf("build mqtt topic for %s: %w", sn, err)
		}
		if address == "" {
			address = topicAddress
		} else if address != topicAddress {
			return "", nil, fmt.Errorf("mqtt broker mismatch: %q vs %q", address, topicAddress)
		}
		targets = append(targets, mqttProbeTarget{
			serialNumber: sn,
			displayName:  displayNameForDevice(devices[i]),
			topic:        topic,
		})
	}
	sort.Slice(targets, func(i, j int) bool {
		return targets[i].serialNumber < targets[j].serialNumber
	})
	return address, targets, nil
}

func displayNameForDevice(device ecoflow.GeneralInfoDevice) string {
	if name := strings.TrimSpace(device.DeviceName); name != "" {
		return name
	}
	if name := strings.TrimSpace(device.ProductName); name != "" {
		return name
	}
	return strings.ToUpper(strings.TrimSpace(device.SN))
}

func buildSmokeMQTTClientID(targets []mqttProbeTarget) string {
	if len(targets) == 0 {
		return ecoflowmqtt.BuildClientIDFromSN("ecoflow-smoke")
	}
	seed := "ecoflow-smoke"
	for i := range targets {
		seed += ":" + targets[i].serialNumber
	}
	seed += ":" + strconv.FormatInt(time.Now().UnixNano(), 36)
	return ecoflowmqtt.BuildClientIDFromSN(seed)
}

func readMQTTProbeLoop(
	ctx context.Context,
	subscriber *ecoflowmqtt.Subscriber,
	targetsByTopic map[string]*mqttProbeTarget,
	targets []mqttProbeTarget,
	statusEvery time.Duration,
	startedAt time.Time,
) error {
	probeCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	msgCh := make(chan ecoflowmqtt.Message)
	errCh := make(chan error, 1)

	go func() {
		for {
			msg, err := subscriber.ReadMessage(probeCtx)
			if err != nil {
				errCh <- err
				return
			}
			select {
			case msgCh <- msg:
			case <-probeCtx.Done():
				return
			}
		}
	}()

	ticker := time.NewTicker(statusEvery)
	defer ticker.Stop()

	pending := countPendingMQTTTargets(targets)
	for pending > 0 {
		select {
		case <-ctx.Done():
			logMQTTProbeStatus("mqtt probe interrupted", targets, startedAt)
			return ctx.Err()
		case err := <-errCh:
			if err == nil {
				continue
			}
			return fmt.Errorf("read mqtt message: %w", err)
		case msg := <-msgCh:
			target, ok := targetsByTopic[msg.Topic]
			if !ok {
				log.Printf("mqtt message on untracked topic: topic=%s bytes=%d", msg.Topic, len(msg.Payload))
				continue
			}
			target.messageCount++
			if !target.seen {
				target.seen = true
				target.seenAt = time.Now()
				pending--
				log.Printf(
					"mqtt live data received: sn=%s name=%q bytes=%d remaining=%d",
					target.serialNumber,
					target.displayName,
					len(msg.Payload),
					pending,
				)
				if pending == 0 {
					log.Printf("mqtt probe complete: all %d devices produced live data", len(targets))
					return nil
				}
				continue
			}
			log.Printf(
				"mqtt additional message: sn=%s name=%q bytes=%d count=%d",
				target.serialNumber,
				target.displayName,
				len(msg.Payload),
				target.messageCount,
			)
		case <-ticker.C:
			logMQTTProbeStatus("mqtt awaiting live data", targets, startedAt)
		}
	}
	return nil
}

func countPendingMQTTTargets(targets []mqttProbeTarget) int {
	pending := 0
	for i := range targets {
		if !targets[i].seen {
			pending++
		}
	}
	return pending
}

func logMQTTProbeStatus(prefix string, targets []mqttProbeTarget, startedAt time.Time) {
	waiting := make([]string, 0, len(targets))
	complete := 0
	for i := range targets {
		if targets[i].seen {
			complete++
			continue
		}
		waiting = append(waiting, fmt.Sprintf("%s(%s)", targets[i].displayName, targets[i].serialNumber))
	}
	log.Printf(
		"%s: seen=%d/%d elapsed=%s waiting=%s",
		prefix,
		complete,
		len(targets),
		time.Since(startedAt).Round(time.Second),
		strings.Join(waiting, ", "),
	)
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
