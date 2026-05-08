package ankersolix

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type Server string

const (
	ServerCOM Server = "com"
	ServerEU  Server = "eu"

	ProviderID = "anker_solix"
)

var serverBaseURLs = map[Server]string{
	ServerCOM: "https://ankerpower-api.anker.com",
	ServerEU:  "https://ankerpower-api-eu.anker.com",
}

type Config struct {
	Server  Server
	Country string

	baseURLOverride string
}

func DefaultConfig() Config {
	return Config{Server: ServerCOM, Country: "US"}
}

func ResolveConfig(server string, country string) (Config, error) {
	cfg := DefaultConfig()
	if strings.TrimSpace(server) != "" {
		cfg.Server = Server(strings.ToLower(strings.TrimSpace(server)))
	}
	if strings.TrimSpace(country) != "" {
		cfg.Country = strings.ToUpper(strings.TrimSpace(country))
	}
	cfg = normalizeConfig(cfg)
	return cfg, cfg.Validate()
}

func normalizeConfig(cfg Config) Config {
	defaults := DefaultConfig()
	if cfg.Server == "" {
		cfg.Server = defaults.Server
	}
	if strings.TrimSpace(cfg.Country) == "" {
		cfg.Country = defaults.Country
	}
	cfg.Country = strings.ToUpper(strings.TrimSpace(cfg.Country))
	return cfg
}

func (c Config) Validate() error {
	if _, ok := serverBaseURLs[c.Server]; !ok {
		return fmt.Errorf("unsupported anker solix server %q", c.Server)
	}
	if !isISO2Country(strings.ToUpper(strings.TrimSpace(c.Country))) {
		return fmt.Errorf("anker solix country must be ISO-2, got %q", c.Country)
	}
	return nil
}

func (c Config) BaseURL() string {
	if strings.TrimSpace(c.baseURLOverride) != "" {
		return strings.TrimRight(strings.TrimSpace(c.baseURLOverride), "/")
	}
	if base := serverBaseURLs[c.Server]; base != "" {
		return base
	}
	return serverBaseURLs[ServerCOM]
}

func isISO2Country(country string) bool {
	country = strings.TrimSpace(country)
	if len(country) != 2 {
		return false
	}
	for _, r := range country {
		if r < 'A' || r > 'Z' {
			return false
		}
	}
	return true
}

type DeviceRef struct {
	ProductCode string
	DeviceSN    string
}

func (r DeviceRef) Normalize() DeviceRef {
	return DeviceRef{
		ProductCode: strings.ToUpper(strings.TrimSpace(r.ProductCode)),
		DeviceSN:    strings.TrimSpace(r.DeviceSN),
	}
}

func (r DeviceRef) ProviderDeviceID() string {
	r = r.Normalize()
	if r.ProductCode == "" || r.DeviceSN == "" {
		return ""
	}
	return strings.ToLower(r.ProductCode) + ":" + r.DeviceSN
}

func ParseProviderDeviceID(raw string) (DeviceRef, error) {
	parts := strings.Split(strings.TrimSpace(raw), ":")
	if len(parts) != 2 {
		return DeviceRef{}, errors.New("anker solix provider_device_id must use product_code:device_sn")
	}
	ref := DeviceRef{ProductCode: parts[0], DeviceSN: parts[1]}.Normalize()
	if ref.ProductCode == "" || ref.DeviceSN == "" {
		return DeviceRef{}, errors.New("anker solix product_code and device_sn are required")
	}
	return ref, nil
}

func CanonicalSN(ref DeviceRef) string {
	ref = ref.Normalize()
	if ref.ProductCode == "" || ref.DeviceSN == "" {
		return ""
	}
	return "ANKER-" + ref.ProductCode + "-" + strings.ToUpper(ref.DeviceSN)
}

type Device struct {
	DeviceSN    string
	ProductCode string
	Name        string
	Alias       string
	DeviceName  string
	AliasName   string
	Firmware    string
	Online      bool
	OwnerUserID string
	SiteID      string
	Raw         map[string]any
}

func (d Device) Ref() DeviceRef {
	return DeviceRef{ProductCode: d.ProductCode, DeviceSN: d.DeviceSN}.Normalize()
}

type NormalizedTelemetry struct {
	ObservedAt   time.Time
	Params       map[string]any
	Capabilities map[string]any
	Metadata     map[string]any
}
