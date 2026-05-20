package pecron

import (
	"encoding/binary"
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

	// Pecron's cloud REST endpoints appear to enforce a per-account daily
	// polling budget of roughly 1280 requests. Keep snapshot refreshes at or
	// above the empirical floor and default to the safer cadence.
	MinCloudRESTPollInterval         = 63 * time.Second
	RecommendedCloudRESTPollInterval = 70 * time.Second
)

type RegionConfig struct {
	ID                       Region
	Name                     string
	BaseURL                  string
	MQTTAddress              string
	MQTTFallbackAddresses    []string
	MQTTPath                 string
	UserDomain               string
	UserDomainSecret         string
	UserDomainFallback       string
	UserDomainSecretFallback string
}

type loginDomain struct {
	UserDomain       string
	UserDomainSecret string
}

var regions = map[Region]RegionConfig{
	RegionUS: {
		ID:                       RegionUS,
		Name:                     "United States",
		BaseURL:                  "https://iot-api.landecia.com",
		MQTTAddress:              "iot-south.landecia.com:8443",
		MQTTPath:                 "/ws/v2",
		UserDomain:               "C.DM.10351.1",
		UserDomainSecret:         "FA5ZHXSka8y9GHvU91Hz1vWvaDSHE2mGW5B7bpn3fXTW",
		UserDomainFallback:       "U.DM.10351.1",
		UserDomainSecretFallback: "HARsQXfeex8vxyaPRAM8fyjqqVuH2uxAGQ3inJ8XxTiB",
	},
	RegionEU: {
		ID:                    RegionEU,
		Name:                  "Europe",
		BaseURL:               "https://iot-api.acceleronix.io",
		MQTTAddress:           "iot-south.acceleronix.io:8443",
		MQTTFallbackAddresses: []string{"iot-south.quecteleu.com:8443"},
		MQTTPath:              "/ws/v2",
		UserDomain:            "C.DM.10351.1",
		UserDomainSecret:      "FA5ZHXSka8y9GHvU91Hz1vWvaDSHE2mGW5B7bpn3fXTW",
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

func (c RegionConfig) MQTTBrokerAddresses() []string {
	return brokerAddressList(c.MQTTAddress, c.MQTTFallbackAddresses)
}

func (c RegionConfig) loginDomains() []loginDomain {
	out := make([]loginDomain, 0, 2)
	if domain := strings.TrimSpace(c.UserDomain); domain != "" {
		out = append(out, loginDomain{
			UserDomain:       domain,
			UserDomainSecret: c.UserDomainSecret,
		})
	}
	if domain := strings.TrimSpace(c.UserDomainFallback); domain != "" {
		out = append(out, loginDomain{
			UserDomain:       domain,
			UserDomainSecret: c.UserDomainSecretFallback,
		})
	}
	return out
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
	pk := strings.TrimSpace(r.ProductKey)
	dk := strings.TrimSpace(r.DeviceKey)
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
		ProductKey: strings.TrimSpace(parts[0]),
		DeviceKey:  strings.TrimSpace(parts[1]),
	}
	if ref.ProductKey == "" || ref.DeviceKey == "" {
		return DeviceRef{}, errors.New("pecron product_key and device_key are required")
	}
	return ref, nil
}

func DeviceRefFromDevice(device Device) DeviceRef {
	return DeviceRef{
		ProductKey: strings.TrimSpace(device.ProductKey),
		DeviceKey:  strings.TrimSpace(device.DeviceKey),
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
	pk := strings.TrimSpace(ref.ProductKey)
	dk := strings.TrimSpace(ref.DeviceKey)
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

func MQTTPublishTopic(ref DeviceRef) string {
	channel := MQTTChannel(ref)
	if channel == "" {
		return ""
	}
	return "q/1/d/" + channel + "/bus"
}

func TTLVReadPacket(packetID uint16) []byte {
	out := make([]byte, 9)
	out[0] = 0xaa
	out[1] = 0xaa
	binary.BigEndian.PutUint16(out[2:4], 5)
	binary.BigEndian.PutUint16(out[5:7], packetID)
	binary.BigEndian.PutUint16(out[7:9], 0x0011)
	out[4] = byte((int(out[5]) + int(out[6]) + int(out[7]) + int(out[8])) & 0xff)
	return out
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
	Address   string
	Addresses []string
	Path      string
	Token     string
	ClientID  string
	Topics    []string
	Ref       DeviceRef
}

func (s MQTTSession) BrokerAddresses() []string {
	return brokerAddressList(s.Address, s.Addresses)
}

func brokerAddressList(primary string, fallbacks []string) []string {
	seen := map[string]struct{}{}
	out := make([]string, 0, 1+len(fallbacks))
	for _, address := range append([]string{primary}, fallbacks...) {
		normalized := strings.TrimSpace(address)
		if normalized == "" {
			continue
		}
		if _, ok := seen[normalized]; ok {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	return out
}
