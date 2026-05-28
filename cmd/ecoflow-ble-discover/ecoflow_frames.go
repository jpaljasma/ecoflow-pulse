package main

import (
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	ecoFlowRFCOMMServiceUUID                  = "00000001-0000-1000-8000-00805f9b34fb"
	ecoFlowRFCOMMWriteCharacteristicUUID      = "00000002-0000-1000-8000-00805f9b34fb"
	ecoFlowRFCOMMNotifyCharacteristicUUID     = "00000003-0000-1000-8000-00805f9b34fb"
	ecoFlowAlternateServiceUUID               = "00000005-0000-1000-8000-00805f9b34fb"
	ecoFlowAlternateWriteCharacteristicUUID   = "00000006-0000-1000-8000-00805f9b34fb"
	ecoFlowAlternateNotifyCharacteristicUUID  = "00000007-0000-1000-8000-00805f9b34fb"
	ecoFlowNordicUARTWriteCharacteristicUUID  = "6e400002-b5a3-f393-e0a9-e50e24dcca9e"
	ecoFlowNordicUARTNotifyCharacteristicUUID = "6e400003-b5a3-f393-e0a9-e50e24dcca9e"
)

type rawBLENotification struct {
	ServiceUUID        string
	CharacteristicUUID string
	Role               string
	Data               []byte
}

type notificationDecode struct {
	Frame          string
	Detail         string
	DecodedHex     string
	DecodedPayload []byte
	Packet         *ecoFlowPacketSummary
	Metrics        []probeMetric
}

type ecoFlowPacketSummary struct {
	Family        string
	Command       string
	PayloadLength int
	Description   string
}

type ecoFlowPacket struct {
	VersionByte   byte
	Version       int
	Sentinel      bool
	Src           byte
	Dst           byte
	DSrc          byte
	DDst          byte
	CmdSet        byte
	CmdID         byte
	Payload       []byte
	HeaderCRCOK   bool
	PacketCRCOK   bool
	PayloadLength int
}

type protoScalar struct {
	FieldNumber int
	WireType    int
	Varint      uint64
	Fixed32     uint32
	Fixed64     uint64
	Bytes       []byte
}

type protoMetricKind int

const (
	protoMetricFloat32 protoMetricKind = iota
	protoMetricOutputFloat32
	protoMetricBatteryInputFloat32
	protoMetricBatteryOutputFloat32
	protoMetricUint
	protoMetricBool
	protoMetricPVState
	protoMetricDCChargingType
)

type delta3DisplayMetricDescriptor struct {
	FieldNumber int
	Name        string
	Unit        string
	Kind        protoMetricKind
}

var delta3DisplayMetricDescriptors = []delta3DisplayMetricDescriptor{
	{FieldNumber: 3, Name: "input_power_w", Unit: "W", Kind: protoMetricFloat32},
	{FieldNumber: 4, Name: "output_power_w", Unit: "W", Kind: protoMetricFloat32},
	{FieldNumber: 54, Name: "ac_input_power_w", Unit: "W", Kind: protoMetricFloat32},
	{FieldNumber: 368, Name: "ac_output_power_w", Unit: "W", Kind: protoMetricOutputFloat32},
	{FieldNumber: 361, Name: "pv_input_power_w", Unit: "W", Kind: protoMetricFloat32},
	{FieldNumber: 70, Name: "pv2_input_power_w", Unit: "W", Kind: protoMetricFloat32},
	{FieldNumber: 363, Name: "pv_input_state", Kind: protoMetricPVState},
	{FieldNumber: 422, Name: "pv2_input_state", Kind: protoMetricPVState},
	{FieldNumber: 11, Name: "usb_c_1_output_power_w", Unit: "W", Kind: protoMetricOutputFloat32},
	{FieldNumber: 12, Name: "usb_c_2_output_power_w", Unit: "W", Kind: protoMetricOutputFloat32},
	{FieldNumber: 100, Name: "usb_c_3_output_power_w", Unit: "W", Kind: protoMetricOutputFloat32},
	{FieldNumber: 9, Name: "usb_a_1_output_power_w", Unit: "W", Kind: protoMetricOutputFloat32},
	{FieldNumber: 10, Name: "usb_a_2_output_power_w", Unit: "W", Kind: protoMetricOutputFloat32},
	{FieldNumber: 37, Name: "dc_12v_output_power_w", Unit: "W", Kind: protoMetricOutputFloat32},
	{FieldNumber: 158, Name: "battery_power_w", Unit: "W", Kind: protoMetricFloat32},
	{FieldNumber: 158, Name: "battery_input_power_w", Unit: "W", Kind: protoMetricBatteryInputFloat32},
	{FieldNumber: 158, Name: "battery_output_power_w", Unit: "W", Kind: protoMetricBatteryOutputFloat32},
	{FieldNumber: 148, Name: "dc_charging_type", Kind: protoMetricDCChargingType},
	{FieldNumber: 262, Name: "battery_soc_percent", Unit: "%", Kind: protoMetricFloat32},
	{FieldNumber: 242, Name: "main_battery_soc_percent", Unit: "%", Kind: protoMetricFloat32},
	{FieldNumber: 268, Name: "battery_discharge_remaining_min", Unit: "min", Kind: protoMetricUint},
	{FieldNumber: 269, Name: "battery_charge_remaining_min", Unit: "min", Kind: protoMetricUint},
	{FieldNumber: 61, Name: "ac_input_plugged", Kind: protoMetricBool},
	{FieldNumber: 202, Name: "ac_charger_connected", Kind: protoMetricBool},
	{FieldNumber: 367, Name: "ac_output_enabled", Kind: protoMetricBool},
	{FieldNumber: 1, Name: "error_code", Kind: protoMetricUint},
}

func inferEcoFlowCharacteristicRole(serviceUUID string, characteristicUUID string) string {
	switch strings.ToLower(strings.TrimSpace(characteristicUUID)) {
	case ecoFlowRFCOMMWriteCharacteristicUUID:
		return "ecoflow_rfcomm_write"
	case ecoFlowRFCOMMNotifyCharacteristicUUID:
		return "ecoflow_rfcomm_notify"
	case ecoFlowAlternateWriteCharacteristicUUID:
		return "ecoflow_alt_write"
	case ecoFlowAlternateNotifyCharacteristicUUID:
		return "ecoflow_alt_notify"
	case ecoFlowNordicUARTWriteCharacteristicUUID:
		return "ecoflow_nordic_uart_write"
	case ecoFlowNordicUARTNotifyCharacteristicUUID:
		return "ecoflow_nordic_uart_notify"
	}
	switch strings.ToLower(strings.TrimSpace(serviceUUID)) {
	case ecoFlowRFCOMMServiceUUID, ecoFlowAlternateServiceUUID:
		return "ecoflow_unknown"
	default:
		return ""
	}
}

func isEcoFlowNotifyRole(role string) bool {
	return strings.HasSuffix(role, "_notify")
}

func probeNotificationFromRaw(raw rawBLENotification, includeRaw bool) (probeNotification, []probeMetric) {
	decoded := decodeEcoFlowNotification(raw.Data)
	notification := probeNotification{
		ServiceUUID:        raw.ServiceUUID,
		CharacteristicUUID: raw.CharacteristicUUID,
		Role:               raw.Role,
		Bytes:              len(raw.Data),
		Frame:              decoded.Frame,
		Detail:             decoded.Detail,
	}
	if decoded.Packet != nil {
		notification.Packet = formatEcoFlowPacketSummary(*decoded.Packet)
	}
	if includeRaw {
		notification.ValueHex = hex.EncodeToString(raw.Data)
		notification.DecodedHex = decoded.DecodedHex
	}
	return notification, decoded.Metrics
}

func decodeEcoFlowNotification(data []byte) notificationDecode {
	if len(data) == 0 {
		return notificationDecode{Frame: "empty", Detail: "empty notification"}
	}
	switch {
	case len(data) >= 2 && data[0] == 0x5a && data[1] == 0x5a:
		return decodeEcoFlowEncPacket(data)
	case data[0] == 0xaa:
		packet, err := parseEcoFlowPacket(data)
		if err != nil {
			return notificationDecode{Frame: "ecoflow_packet", Detail: err.Error()}
		}
		return decodeEcoFlowPacketMetrics(packet, "ecoflow_packet")
	default:
		return notificationDecode{Frame: "unknown", Detail: "unknown notification prefix"}
	}
}

func decodeEcoFlowEncPacket(data []byte) notificationDecode {
	if len(data) < 8 {
		return notificationDecode{Frame: "ecoflow_enc_packet", Detail: "too short"}
	}
	payloadLength := int(binary.LittleEndian.Uint16(data[4:6]))
	if payloadLength < 2 {
		return notificationDecode{Frame: "ecoflow_enc_packet", Detail: "invalid payload length"}
	}
	end := 6 + payloadLength
	if end > len(data) {
		return notificationDecode{Frame: "ecoflow_enc_packet", Detail: "incomplete frame"}
	}
	payload := data[6 : end-2]
	decodedHex := hex.EncodeToString(payload)
	wantCRC := binary.LittleEndian.Uint16(data[end-2 : end])
	if gotCRC := ecoFlowCRC16(data[:end-2]); gotCRC != wantCRC {
		return notificationDecode{Frame: "ecoflow_enc_packet", Detail: "crc16 mismatch"}
	}
	frameType := data[2] >> 4
	detail := "frame_type=" + describeEcoFlowEncFrameType(frameType)
	if len(payload) == 0 || payload[0] != 0xaa {
		simpleDetail := ""
		if frameType == 0x00 {
			simpleDetail = describeEcoFlowSimplePayload(payload)
		}
		if simpleDetail == "" {
			simpleDetail = "encrypted_or_unknown_payload"
		}
		return notificationDecode{
			Frame:          "ecoflow_enc_packet",
			Detail:         joinDetails(detail, simpleDetail),
			DecodedHex:     decodedHex,
			DecodedPayload: append([]byte(nil), payload...),
		}
	}
	packet, err := parseEcoFlowPacket(payload)
	if err != nil {
		return notificationDecode{
			Frame:          "ecoflow_enc_packet",
			Detail:         joinDetails(detail, "encrypted_or_unknown_payload"),
			DecodedHex:     decodedHex,
			DecodedPayload: append([]byte(nil), payload...),
		}
	}
	decoded := decodeEcoFlowPacketMetrics(packet, "ecoflow_enc_packet")
	decoded.Frame = "ecoflow_enc_packet"
	decoded.DecodedHex = decodedHex
	decoded.DecodedPayload = append([]byte(nil), payload...)
	if decoded.Detail == "" {
		decoded.Detail = detail
	} else {
		decoded.Detail = detail + ", " + decoded.Detail
	}
	return decoded
}

func describeEcoFlowEncFrameType(frameType byte) string {
	switch frameType {
	case 0x00:
		return "command"
	case 0x01:
		return "protocol"
	case 0x10:
		return "protocol_int"
	default:
		return fmt.Sprintf("0x%02x", frameType)
	}
}

func joinDetails(parts ...string) string {
	nonEmpty := make([]string, 0, len(parts))
	for _, part := range parts {
		if part != "" {
			nonEmpty = append(nonEmpty, part)
		}
	}
	return strings.Join(nonEmpty, ", ")
}

func describeEcoFlowSimplePayload(payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	switch payload[0] {
	case 0x01:
		if len(payload) >= 42 {
			return "simple=ecdh_public_key"
		}
		return "simple=ecdh_public_key_short"
	case 0x02:
		return "simple=session_key_info"
	default:
		return ""
	}
}

func decodeEcoFlowPacketMetrics(packet ecoFlowPacket, frame string) notificationDecode {
	summary := summarizeEcoFlowPacket(packet)
	decoded := notificationDecode{
		Frame:          frame,
		DecodedHex:     hex.EncodeToString(packet.Payload),
		DecodedPayload: append([]byte(nil), packet.Payload...),
		Packet:         &summary,
	}
	if !packet.HeaderCRCOK {
		decoded.Detail = "header_crc8_mismatch"
		return decoded
	}
	if !packet.PacketCRCOK {
		decoded.Detail = "packet_crc16_mismatch"
		return decoded
	}
	if packet.Src == 0x02 && packet.CmdSet == 0xfe && packet.CmdID == 0x15 {
		decoded.Metrics = decodeDelta3DisplayUploadMetrics(packet.Payload)
	}
	if packet.CmdSet == 0x35 && packet.CmdID == 0x86 && packet.Src != 0x21 {
		decoded.Metrics = append(decoded.Metrics, probeMetric{
			Name:   "auth_result",
			Value:  describeEcoFlowAuthResult(packet.Payload),
			Source: "ble_auth",
		})
	}
	return decoded
}

func summarizeEcoFlowPacket(packet ecoFlowPacket) ecoFlowPacketSummary {
	family := "v" + strconv.Itoa(packet.Version)
	if packet.Sentinel {
		family += "_sentinel"
	}
	description := ""
	if packet.Src == 0x02 && packet.CmdSet == 0xfe && packet.CmdID == 0x15 {
		description = "v3 display property upload"
	}
	if packet.CmdSet == 0x35 && packet.CmdID == 0x86 {
		description = "auth response"
		if packet.Src == 0x21 && packet.Dst == 0x35 {
			description = "auth request"
		}
	}
	if packet.Src == 0x35 && packet.CmdSet == 0x35 && packet.CmdID == 0x20 {
		description = "ble ping"
	}
	if packet.CmdSet == 0x01 {
		switch packet.CmdID {
		case 0x52:
			if packet.Src == 0x35 && len(packet.Payload) == 0 {
				description = "device time request"
			} else {
				description = "rtc time response"
			}
		case 0x53:
			description = "rtc time check"
		case 0x55:
			description = "utc time sync"
		}
	}
	return ecoFlowPacketSummary{
		Family:        family,
		Command:       fmt.Sprintf("src=0x%02x,dst=0x%02x,cmd_set=0x%02x,cmd_id=0x%02x", packet.Src, packet.Dst, packet.CmdSet, packet.CmdID),
		PayloadLength: len(packet.Payload),
		Description:   description,
	}
}

func describeEcoFlowAuthResult(payload []byte) string {
	if len(payload) == 0 {
		return "empty"
	}
	switch payload[0] {
	case 0x00:
		return "ok"
	case 0x01:
		return "need_refresh_token"
	case 0x02:
		return "device_internal_error"
	case 0x03:
		return "device_already_bound"
	case 0x04:
		return "need_bind_install_first"
	case 0x05:
		return "app_send_data_error"
	case 0x06:
		return "wrong_key"
	case 0x07:
		return "maximum_devices"
	default:
		return fmt.Sprintf("unknown_%02x", payload[0])
	}
}

func formatEcoFlowPacketSummary(summary ecoFlowPacketSummary) string {
	parts := []string{
		summary.Family,
		summary.Command,
		"payload=" + strconv.Itoa(summary.PayloadLength),
	}
	if summary.Description != "" {
		parts = append(parts, "description="+strings.ReplaceAll(summary.Description, " ", "_"))
	}
	return strings.Join(parts, ",")
}

func parseEcoFlowPacket(data []byte) (ecoFlowPacket, error) {
	if len(data) < 5 || data[0] != 0xaa {
		return ecoFlowPacket{}, errorsText("invalid packet prefix")
	}
	versionByte := data[1]
	if versionByte == 0x04 {
		return ecoFlowPacket{}, errorsText("v4 packet parsing is not implemented")
	}
	version := int(versionByte & 0x0f)
	if version != 2 && version != 3 {
		return ecoFlowPacket{}, fmt.Errorf("unsupported packet version 0x%02x", versionByte)
	}
	payloadLength := int(binary.LittleEndian.Uint16(data[2:4]))
	payloadStart := 16
	if version == 3 {
		payloadStart = 18
	}
	if len(data) < payloadStart {
		return ecoFlowPacket{}, errorsText("packet too short")
	}
	sentinel := versionByte&0x10 != 0
	totalLength := payloadStart + payloadLength
	if !sentinel {
		totalLength += 2
	}
	if len(data) < totalLength {
		return ecoFlowPacket{}, errorsText("packet length mismatch")
	}
	packet := ecoFlowPacket{
		VersionByte:   versionByte,
		Version:       version,
		Sentinel:      sentinel,
		Src:           data[12],
		Dst:           data[13],
		PayloadLength: payloadLength,
		HeaderCRCOK:   ecoFlowCRC8(data[:4]) == data[4],
		PacketCRCOK:   true,
	}
	if version == 2 {
		packet.CmdSet = data[14]
		packet.CmdID = data[15]
	} else {
		packet.DSrc = data[14]
		packet.DDst = data[15]
		packet.CmdSet = data[16]
		packet.CmdID = data[17]
	}
	packet.Payload = append([]byte(nil), data[payloadStart:payloadStart+payloadLength]...)
	if sentinel {
		seqXOR := data[6]
		if seqXOR != 0 {
			for i := range packet.Payload {
				packet.Payload[i] ^= seqXOR
			}
		}
		if len(packet.Payload) >= 2 && packet.Payload[len(packet.Payload)-2] == 0xbb && packet.Payload[len(packet.Payload)-1] == 0xbb {
			packet.Payload = packet.Payload[:len(packet.Payload)-2]
		}
		return packet, nil
	}
	packet.PacketCRCOK = ecoFlowCRC16(data[:totalLength-2]) == binary.LittleEndian.Uint16(data[totalLength-2:totalLength])
	return packet, nil
}

func decodeDelta3DisplayUploadMetrics(payload []byte) []probeMetric {
	fields, err := parseProtoScalars(payload)
	if err != nil {
		return nil
	}
	values := make(map[int]protoScalar, len(fields))
	for _, field := range fields {
		values[field.FieldNumber] = field
	}

	metrics := make([]probeMetric, 0, len(delta3DisplayMetricDescriptors))
	for _, descriptor := range delta3DisplayMetricDescriptors {
		value, ok := values[descriptor.FieldNumber]
		if !ok {
			continue
		}
		metricValue, ok := formatDelta3MetricValue(descriptor, value)
		if !ok {
			continue
		}
		metric := probeMetric{
			Name:    descriptor.Name,
			Value:   metricValue,
			Unit:    descriptor.Unit,
			Source:  "ble_notify",
			Decoder: "v3_display",
		}
		metrics = append(metrics, metric)
	}
	return metrics
}

func formatDelta3MetricValue(descriptor delta3DisplayMetricDescriptor, value protoScalar) (string, bool) {
	switch descriptor.Kind {
	case protoMetricFloat32:
		if value.WireType != 5 {
			return "", false
		}
		return formatFloat32Metric(math.Float32frombits(value.Fixed32)), true
	case protoMetricOutputFloat32:
		if value.WireType != 5 {
			return "", false
		}
		raw := math.Float32frombits(value.Fixed32)
		if raw != 0 {
			raw = -raw
		}
		return formatFloat32Metric(raw), true
	case protoMetricBatteryInputFloat32:
		if value.WireType != 5 {
			return "", false
		}
		return formatFloat32Metric(max(math.Float32frombits(value.Fixed32), 0)), true
	case protoMetricBatteryOutputFloat32:
		if value.WireType != 5 {
			return "", false
		}
		return formatFloat32Metric(-min(math.Float32frombits(value.Fixed32), 0)), true
	case protoMetricUint:
		if value.WireType != 0 {
			return "", false
		}
		return strconv.FormatUint(value.Varint, 10), true
	case protoMetricBool:
		if value.WireType != 0 {
			return "", false
		}
		if value.Varint == 0 {
			return "false", true
		}
		return "true", true
	case protoMetricPVState:
		if value.WireType != 0 {
			return "", false
		}
		return describeDelta3PVState(value.Varint), true
	case protoMetricDCChargingType:
		if value.WireType != 0 {
			return "", false
		}
		return describeEcoFlowDCChargingType(value.Varint), true
	default:
		return "", false
	}
}

func formatFloat32Metric(value float32) string {
	return strconv.FormatFloat(float64(value), 'f', -1, 32)
}

func describeDelta3PVState(value uint64) string {
	switch value {
	case 0:
		return "off"
	case 1:
		return "car"
	case 2:
		return "solar"
	default:
		return "unknown_" + strconv.FormatUint(value, 10)
	}
}

func describeEcoFlowDCChargingType(value uint64) string {
	switch value {
	case 0:
		return "auto"
	case 1:
		return "car"
	case 2:
		return "solar"
	default:
		return "unknown_" + strconv.FormatUint(value, 10)
	}
}

func parseProtoScalars(data []byte) ([]protoScalar, error) {
	fields := make([]protoScalar, 0)
	for len(data) > 0 {
		key, n, ok := readProtoVarint(data)
		if !ok {
			return nil, errorsText("invalid protobuf field key")
		}
		data = data[n:]
		fieldNumber := int(key >> 3)
		wireType := int(key & 0x07)
		if fieldNumber <= 0 {
			return nil, errorsText("invalid protobuf field number")
		}
		field := protoScalar{FieldNumber: fieldNumber, WireType: wireType}
		switch wireType {
		case 0:
			value, n, ok := readProtoVarint(data)
			if !ok {
				return nil, errorsText("invalid protobuf varint")
			}
			field.Varint = value
			data = data[n:]
		case 1:
			if len(data) < 8 {
				return nil, errorsText("invalid protobuf fixed64")
			}
			field.Fixed64 = binary.LittleEndian.Uint64(data[:8])
			data = data[8:]
		case 2:
			length, n, ok := readProtoVarint(data)
			if !ok || length > uint64(len(data[n:])) {
				return nil, errorsText("invalid protobuf bytes")
			}
			data = data[n:]
			field.Bytes = append([]byte(nil), data[:length]...)
			data = data[length:]
		case 5:
			if len(data) < 4 {
				return nil, errorsText("invalid protobuf fixed32")
			}
			field.Fixed32 = binary.LittleEndian.Uint32(data[:4])
			data = data[4:]
		default:
			return nil, fmt.Errorf("unsupported protobuf wire type %d", wireType)
		}
		fields = append(fields, field)
	}
	return fields, nil
}

func readProtoVarint(data []byte) (uint64, int, bool) {
	var value uint64
	for i, b := range data {
		if i == 10 {
			return 0, 0, false
		}
		value |= uint64(b&0x7f) << (7 * i)
		if b < 0x80 {
			return value, i + 1, true
		}
	}
	return 0, 0, false
}

func ecoFlowCRC8(data []byte) byte {
	var crc byte
	for _, b := range data {
		crc ^= b
		for i := 0; i < 8; i++ {
			if crc&0x80 != 0 {
				crc = (crc << 1) ^ 0x07
			} else {
				crc <<= 1
			}
		}
	}
	return crc
}

func ecoFlowCRC16(data []byte) uint16 {
	var crc uint16
	for _, b := range data {
		crc ^= uint16(b)
		for i := 0; i < 8; i++ {
			if crc&0x0001 != 0 {
				crc = (crc >> 1) ^ 0xa001
			} else {
				crc >>= 1
			}
		}
	}
	return crc
}

func errorsText(message string) error {
	return errors.New(message)
}
