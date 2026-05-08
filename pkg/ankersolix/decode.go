package ankersolix

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
)

const (
	tlvTypeString byte = 0x00
	tlvTypeUI     byte = 0x01
	tlvTypeSILE   byte = 0x02
	tlvTypeVar    byte = 0x03
	tlvTypeBin    byte = 0x04
	tlvTypeSFLE   byte = 0x05
	tlvTypeJSON   byte = 0xfe
)

type MQTTMessage struct {
	ProductCode string
	DeviceSN    string
	Topic       string
	MessageType string
	Data        []byte
	Values      map[string]any
	RawPayload  map[string]any
}

type DecodedPayload struct {
	ProductCode string
	MessageType string
	Values      map[string]any
	RawFields   map[string]any
}

func DecodeMQTTWrapper(topic string, payload []byte) (MQTTMessage, error) {
	var outer map[string]any
	if err := json.Unmarshal(payload, &outer); err != nil {
		return MQTTMessage{}, fmt.Errorf("decode anker solix mqtt wrapper: %w", err)
	}
	rawPayload, err := decodeInnerPayload(outer["payload"])
	if err != nil {
		return MQTTMessage{}, err
	}
	productCode := strings.ToUpper(firstNonEmpty(
		asString(rawPayload["pn"]),
		asString(rawPayload["device_pn"]),
		topicPart(topic, 2),
		asString(asMap(outer["head"])["device_pn"]),
	))
	deviceSN := firstNonEmpty(
		asString(rawPayload["sn"]),
		asString(rawPayload["device_sn"]),
		topicPart(topic, 3),
		asString(asMap(outer["head"])["device_sn"]),
	)
	var data []byte
	var isJSON bool
	if encoded := asString(rawPayload["data"]); encoded != "" {
		data, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return MQTTMessage{}, fmt.Errorf("decode anker solix mqtt data: %w", err)
		}
	} else if encoded := asString(rawPayload["trans"]); encoded != "" {
		data, err = base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return MQTTMessage{}, fmt.Errorf("decode anker solix mqtt trans: %w", err)
		}
		isJSON = true
	}
	message := MQTTMessage{
		ProductCode: productCode,
		DeviceSN:    deviceSN,
		Topic:       topic,
		Data:        append([]byte(nil), data...),
		RawPayload:  rawPayload,
	}
	if len(data) == 0 {
		return message, nil
	}
	var decoded DecodedPayload
	if isJSON || json.Valid(data) {
		decoded, err = DecodeJSONPayload(productCode, data)
	} else {
		decoded, err = DecodeBinaryPayload(productCode, data)
	}
	if err != nil {
		return message, err
	}
	message.MessageType = decoded.MessageType
	message.Values = decoded.Values
	return message, nil
}

func decodeInnerPayload(value any) (map[string]any, error) {
	switch v := value.(type) {
	case map[string]any:
		return v, nil
	case string:
		var out map[string]any
		if err := json.Unmarshal([]byte(v), &out); err != nil {
			return nil, fmt.Errorf("decode anker solix mqtt inner payload: %w", err)
		}
		return out, nil
	case json.RawMessage:
		var asStringPayload string
		if err := json.Unmarshal(v, &asStringPayload); err == nil {
			return decodeInnerPayload(asStringPayload)
		}
		var out map[string]any
		if err := json.Unmarshal(v, &out); err != nil {
			return nil, fmt.Errorf("decode anker solix mqtt inner payload: %w", err)
		}
		return out, nil
	default:
		return nil, errors.New("anker solix mqtt payload is missing")
	}
}

func DecodeBinaryPayload(productCode string, data []byte) (DecodedPayload, error) {
	if len(data) < 10 {
		return DecodedPayload{}, errors.New("anker solix binary payload too short")
	}
	if data[0] != 0xff || data[1] != 0x09 {
		return DecodedPayload{}, errors.New("anker solix binary payload marker mismatch")
	}
	productCode = strings.ToUpper(strings.TrimSpace(productCode))
	messageType := hex.EncodeToString(data[7:9])
	idx := 9
	if idx < len(data)-1 && data[idx] < 0xa0 {
		idx++
	}
	values := map[string]any{}
	rawFields := map[string]any{}
	limit := len(data) - 1
	for idx+2 <= limit {
		fieldID := hex.EncodeToString(data[idx : idx+1])
		length := int(data[idx+1])
		idx += 2
		if length <= 0 || idx+length > limit {
			break
		}
		field := data[idx : idx+length]
		idx += length
		typ := byte(0xff)
		valueBytes := field
		if len(field) > 0 {
			typ = field[0]
			valueBytes = field[1:]
		}
		value, ok := decodeTLVValue(typ, valueBytes)
		if !ok {
			continue
		}
		name := fieldName(productCode, messageType, fieldID)
		if name == "" {
			name = "field_" + fieldID
		}
		rawFields[fieldID] = value
		values[name] = value
	}
	return DecodedPayload{
		ProductCode: productCode,
		MessageType: messageType,
		Values:      values,
		RawFields:   rawFields,
	}, nil
}

func DecodeJSONPayload(productCode string, data []byte) (DecodedPayload, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return DecodedPayload{}, fmt.Errorf("decode anker solix JSON payload: %w", err)
	}
	values := map[string]any{}
	for key, value := range raw {
		name := jsonFieldName(key)
		values[name] = value
	}
	return DecodedPayload{
		ProductCode: strings.ToUpper(strings.TrimSpace(productCode)),
		MessageType: "json",
		Values:      values,
		RawFields:   raw,
	}, nil
}

func decodeTLVValue(typ byte, raw []byte) (any, bool) {
	switch typ {
	case tlvTypeString:
		return strings.TrimRight(string(raw), "\x00"), true
	case tlvTypeUI:
		if len(raw) == 0 {
			return nil, false
		}
		return float64(raw[0]), true
	case tlvTypeSILE:
		if len(raw) < 2 {
			return nil, false
		}
		return float64(int16(binary.LittleEndian.Uint16(raw[:2]))), true
	case tlvTypeVar:
		if len(raw) == 0 {
			return nil, false
		}
		buf := make([]byte, 4)
		copy(buf, raw)
		return float64(int32(binary.LittleEndian.Uint32(buf))), true
	case tlvTypeSFLE:
		if len(raw) < 4 {
			return nil, false
		}
		return float64(math.Float32frombits(binary.LittleEndian.Uint32(raw[:4]))), true
	case tlvTypeJSON:
		var out any
		if err := json.Unmarshal(raw, &out); err != nil {
			return nil, false
		}
		return out, true
	case tlvTypeBin:
		return append([]byte(nil), raw...), true
	default:
		return append([]byte(nil), raw...), true
	}
}

func fieldName(productCode string, messageType string, fieldID string) string {
	key := productCode + ":" + messageType + ":" + fieldID
	if name := fieldNames[key]; name != "" {
		return name
	}
	key = productCode + ":*:" + fieldID
	if name := fieldNames[key]; name != "" {
		return name
	}
	key = "*:" + messageType + ":" + fieldID
	if name := fieldNames[key]; name != "" {
		return name
	}
	return fieldNames["*:*:"+fieldID]
}

var fieldNames = map[string]string{
	"*:*:a2": "device_sn",
	"*:*:aa": "temperature",

	"A1722:0405:bb": "battery_soc",
	"A1722:0405:ac": "dc_input_power_total",
	"A1722:0405:ad": "ac_input_power_total",
	"A1722:0405:ae": "ac_output_power_total",

	"A1723:0405:bb": "battery_soc",
	"A1723:0405:ac": "dc_input_power_total",
	"A1723:0405:ad": "ac_input_power_total",
	"A1723:0405:ae": "ac_output_power_total",

	"A1725:0405:b7": "battery_soc",
	"A1725:0405:ac": "dc_input_power_total",
	"A1725:0405:ad": "dc_output_power_total",
	"A1726:0405:b7": "battery_soc",
	"A1726:0405:ac": "dc_input_power_total",
	"A1726:0405:ad": "dc_output_power_total",
	"A1727:0405:b7": "battery_soc",
	"A1727:0405:ac": "dc_input_power_total",
	"A1727:0405:ad": "dc_output_power_total",
	"A1728:0405:b7": "battery_soc",
	"A1728:0405:ac": "dc_input_power_total",
	"A1728:0405:ad": "dc_output_power_total",
	"A1729:0405:b7": "battery_soc",
	"A1729:0405:ac": "dc_input_power_total",
	"A1729:0405:ad": "dc_output_power_total",

	"A1761:*:c1":  "main_battery_soc",
	"A1761:*:ae":  "dc_input_power",
	"A1761:*:af":  "photovoltaic_power",
	"A1761:*:b0":  "output_power_total",
	"A1780:*:c1":  "main_battery_soc",
	"A1780:*:ae":  "dc_input_power",
	"A1780:*:af":  "photovoltaic_power",
	"A1780:*:b0":  "output_power_total",
	"A1780P:*:c1": "main_battery_soc",
	"A1780P:*:ae": "dc_input_power",
	"A1780P:*:af": "photovoltaic_power",
	"A1780P:*:b0": "output_power_total",

	"A1763:*:a5": "battery_soc",
	"A1763:*:a6": "output_power_total",
	"A1763:*:a7": "dc_input_power_total",
	"A1763:*:b2": "dc_output_power_total",
	"A1782:*:a5": "battery_soc",
	"A1782:*:a6": "output_power_total",
	"A1782:*:a7": "dc_input_power_total",
	"A1782:*:b2": "dc_output_power_total",
	"A1783:*:a5": "battery_soc",
	"A1783:*:a6": "output_power_total",
	"A1783:*:a7": "dc_input_power_total",
	"A1783:*:b2": "dc_output_power_total",

	"A1790:*:c0":  "battery_soc",
	"A1790:*:af":  "pv_1_power",
	"A1790:*:b0":  "pv_2_power",
	"A1790:*:b2":  "output_power_total",
	"A1790P:*:c0": "battery_soc",
	"A1790P:*:af": "pv_1_power",
	"A1790P:*:b0": "pv_2_power",
	"A1790P:*:b2": "output_power_total",

	"A17C0:*:a3": "battery_soc",
	"A17C0:*:ab": "photovoltaic_power",
	"A17C0:*:ac": "ac_output_power",
	"A17C1:*:b0": "battery_soc",
	"A17C1:*:ce": "pv_1_power",
	"A17C1:*:cf": "pv_2_power",
	"A17C1:*:c8": "home_demand",
	"A17C2:*:b0": "battery_soc",
	"A17C2:*:ce": "pv_1_power",
	"A17C2:*:cf": "pv_2_power",
	"A17C2:*:c8": "home_demand",
	"A17C3:*:b0": "battery_soc",
	"A17C3:*:ce": "pv_1_power",
	"A17C3:*:cf": "pv_2_power",
	"A17C3:*:c8": "home_demand",
	"A17C5:*:a6": "battery_soc",
	"A17C5:*:c6": "pv_1_power",
	"A17C5:*:c7": "pv_2_power",
	"A17C5:*:c8": "pv_3_power",
	"A17C5:*:c9": "pv_4_power",
	"A17C5:*:c5": "home_demand",
	"A17C5:*:c4": "grid_power_signed",

	"A17E1:*:a3": "battery_soc",
	"A17E1:*:c4": "grid_power_signed",
	"A17E1:*:c5": "home_demand",
	"A17E1:*:c6": "pv_1_power",
	"A17E1:*:c7": "pv_2_power",
	"AX170:*:a6": "battery_soc_total",
	"AX170:*:ab": "pv_power_total",
	"AX170:*:b3": "battery_power_signed",
	"AX170:*:c4": "grid_power_signed",

	"A17B1:*:a6": "battery_soc_total",
	"A17B1:*:ab": "pv_power_total",
	"A17B1:*:af": "home_demand",
	"A17B1:*:c4": "grid_power_signed",
}

func jsonFieldName(key string) string {
	switch strings.TrimSpace(key) {
	case "soc":
		return "battery_soc"
	case "pp":
		return "photovoltaic_power"
	case "gp":
		return "grid_power_signed"
	case "g2lp":
		return "grid_to_home_power"
	case "lp":
		return "home_demand"
	case "bp":
		return "battery_power_signed"
	case "b2lp":
		return "battery_to_home_power"
	case "p2lp":
		return "pv_to_home_power"
	case "p2bp":
		return "pv_to_battery_power"
	case "p2gp":
		return "pv_to_grid_power"
	case "g2bp":
		return "grid_to_battery_power"
	case "bds":
		return "battery_modules"
	default:
		return strings.TrimSpace(key)
	}
}

func topicPart(topic string, idx int) string {
	parts := strings.Split(topic, "/")
	if idx < 0 || idx >= len(parts) {
		return ""
	}
	return strings.TrimSpace(parts[idx])
}

func mustHex(value string) []byte {
	decoded, err := hex.DecodeString(strings.TrimSpace(value))
	if err != nil {
		panic(err)
	}
	return decoded
}
