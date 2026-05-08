package pecron

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Region string

const (
	RegionUS Region = "us"
	RegionEU Region = "eu"
	RegionCN Region = "cn"

	ProductKeyE1000LFP = "p11vxg"
)

type RegionConfig struct {
	ID               Region
	Name             string
	BaseURL          string
	MQTTAddress      string
	MQTTPath         string
	UserDomain       string
	UserDomainSecret string
}

var regions = map[Region]RegionConfig{
	RegionUS: {
		ID:               RegionUS,
		Name:             "United States",
		BaseURL:          "https://iot-api.landecia.com",
		MQTTAddress:      "iot-south.landecia.com:8443",
		MQTTPath:         "/ws/v2",
		UserDomain:       "U.DM.10351.1",
		UserDomainSecret: "HARsQXfeex8vxyaPRAM8fyjqqVuH2uxAGQ3inJ8XxTiB",
	},
	RegionEU: {
		ID:               RegionEU,
		Name:             "Europe",
		BaseURL:          "https://iot-api.acceleronix.io",
		MQTTAddress:      "iot-south.acceleronix.io:8443",
		MQTTPath:         "/ws/v2",
		UserDomain:       "C.DM.10351.1",
		UserDomainSecret: "FA5ZHXSka8y9GHvU91Hz1vWvaDSHE2mGW5B7bpn3fXTW",
	},
	RegionCN: {
		ID:               RegionCN,
		Name:             "China",
		BaseURL:          "https://iot-api.quectelcn.com",
		MQTTAddress:      "iot-south.quectelcn.com:8443",
		MQTTPath:         "/ws/v2",
		UserDomain:       "C.DM.5903.1",
		UserDomainSecret: "EufftRJSuWuVY7c6txzGifV9bJcfXHAFa7hXY5doXSn7",
	},
}

func ResolveRegion(raw string) (RegionConfig, error) {
	normalized := strings.ToLower(strings.TrimSpace(raw))
	switch normalized {
	case "", "us", "usa", "na", "north-america", "north_america":
		normalized = string(RegionUS)
	case "eu", "europe":
		normalized = string(RegionEU)
	case "cn", "china":
		normalized = string(RegionCN)
	}
	cfg, ok := regions[Region(normalized)]
	if !ok {
		return RegionConfig{}, fmt.Errorf("unsupported pecron region %q", raw)
	}
	return cfg, nil
}

type Session struct {
	AccessToken  string
	RefreshToken string
	UserID       string
	ExpiresAt    time.Time
}

func (s Session) NeedsRefresh(now time.Time, skew time.Duration) bool {
	if strings.TrimSpace(s.AccessToken) == "" {
		return true
	}
	if s.ExpiresAt.IsZero() {
		return false
	}
	if skew < 0 {
		skew = 0
	}
	return !now.Add(skew).Before(s.ExpiresAt)
}

type Device struct {
	DeviceName     string
	ProductKey     string
	DeviceKey      string
	ProductName    string
	Online         bool
	Protocol       string
	DeviceSN       string
	Firmware       string
	MCUVersion     string
	SignalStrength int
	LastConnTime   string
}

type DeviceRef struct {
	ProductKey string
	DeviceKey  string
}

func (r DeviceRef) ProviderDeviceID() string {
	pk := strings.ToLower(strings.TrimSpace(r.ProductKey))
	dk := strings.ToLower(strings.TrimSpace(r.DeviceKey))
	if pk == "" || dk == "" {
		return ""
	}
	return pk + ":" + dk
}

func ParseProviderDeviceID(raw string) (DeviceRef, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return DeviceRef{}, errors.New("pecron provider_device_id must use product_key:device_key")
	}
	ref := DeviceRef{
		ProductKey: strings.ToLower(strings.TrimSpace(parts[0])),
		DeviceKey:  strings.ToLower(strings.TrimSpace(parts[1])),
	}
	if ref.ProductKey == "" || ref.DeviceKey == "" {
		return DeviceRef{}, errors.New("pecron product_key and device_key are required")
	}
	return ref, nil
}

func DeviceRefFromDevice(device Device) DeviceRef {
	return DeviceRef{
		ProductKey: strings.ToLower(strings.TrimSpace(device.ProductKey)),
		DeviceKey:  strings.ToLower(strings.TrimSpace(device.DeviceKey)),
	}
}

func CanonicalSN(ref DeviceRef) string {
	pk := strings.ToUpper(strings.TrimSpace(ref.ProductKey))
	dk := strings.ToUpper(strings.TrimSpace(ref.DeviceKey))
	if pk == "" || dk == "" {
		return ""
	}
	return "PECRON-" + pk + "-" + dk
}

func MQTTChannel(ref DeviceRef) string {
	pk := strings.ToLower(strings.TrimSpace(ref.ProductKey))
	dk := strings.ToLower(strings.TrimSpace(ref.DeviceKey))
	if pk == "" || dk == "" {
		return ""
	}
	return "qd" + pk + dk
}

func MQTTSubscribeTopics(ref DeviceRef) []string {
	channel := MQTTChannel(ref)
	if channel == "" {
		return nil
	}
	return []string{
		"q/2/d/" + channel + "/bus_",
		"q/2/d/" + channel + "/ack_",
		"q/2/d/" + channel + "/onl_",
	}
}

type NormalizedTelemetry struct {
	Params       map[string]any
	Capabilities map[string]any
	Metadata     map[string]any
}

type TSLProperty struct {
	Code       string
	Name       string
	DataType   string
	AccessMode string
	Writable   bool
	Raw        map[string]any
}

type MQTTSession struct {
	Address  string
	Path     string
	Token    string
	ClientID string
	Topics   []string
}
