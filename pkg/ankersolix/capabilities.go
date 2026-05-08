package ankersolix

import "strings"

type ModelFamily string

const (
	FamilyPowerStation ModelFamily = "power_station"
	FamilySolarbank    ModelFamily = "solarbank"
	FamilyHomeBackup   ModelFamily = "home_backup"
	FamilyPowerPanel   ModelFamily = "home_power_panel"
	FamilyHES          ModelFamily = "hes"
	FamilyGenerator    ModelFamily = "generator"
	FamilyExcluded     ModelFamily = "excluded"
)

type SupportStatus string

const (
	SupportEnabled     SupportStatus = "enabled"
	SupportCompanion   SupportStatus = "companion"
	SupportNeedsSample SupportStatus = "needs_sample"
	SupportExcluded    SupportStatus = "excluded"
)

type ModelCapability struct {
	ProductCode       string
	DisplayName       string
	Family            ModelFamily
	Status            SupportStatus
	BatteryCapacityWh int
	DefaultPVInputs   int
	MQTTMessageTypes  []string
}

func (c ModelCapability) Enableable() bool {
	return c.Status == SupportEnabled
}

func (c ModelCapability) SupportedTelemetry() bool {
	return c.Status == SupportEnabled || c.Status == SupportCompanion
}

func LookupCapability(productCode string) (ModelCapability, bool) {
	code := strings.ToUpper(strings.TrimSpace(productCode))
	capability, ok := modelRegistry[code]
	return capability, ok
}

func IsEnableableProduct(productCode string) bool {
	capability, ok := LookupCapability(productCode)
	return ok && capability.Enableable()
}

func IsExcludedProduct(productCode string) bool {
	capability, ok := LookupCapability(productCode)
	return ok && capability.Status == SupportExcluded
}

var modelRegistry = map[string]ModelCapability{}

func register(capability ModelCapability) {
	capability.ProductCode = strings.ToUpper(strings.TrimSpace(capability.ProductCode))
	if capability.ProductCode == "" {
		return
	}
	modelRegistry[capability.ProductCode] = capability
}

func init() {
	for _, capability := range []ModelCapability{
		{ProductCode: "A1722", DisplayName: "SOLIX C300 AC", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 288, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0405"}},
		{ProductCode: "A1723", DisplayName: "SOLIX C300X AC", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 288, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0405"}},
		{ProductCode: "A1725", DisplayName: "SOLIX C200", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 192, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0401", "0405"}},
		{ProductCode: "A1726", DisplayName: "SOLIX C300 DC", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 288, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0401", "0405"}},
		{ProductCode: "A1727", DisplayName: "SOLIX C200 DC", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 192, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0401", "0405"}},
		{ProductCode: "A1728", DisplayName: "SOLIX C300X DC", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 288, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0401", "0405"}},
		{ProductCode: "A1729", DisplayName: "SOLIX C200X DC", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 192, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0401", "0405"}},
		{ProductCode: "A1761", DisplayName: "SOLIX C1000", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 1056, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0405"}},
		{ProductCode: "A1763", DisplayName: "SOLIX C1000 Gen 2", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 1024, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0421", "0900"}},
		{ProductCode: "A1780", DisplayName: "SOLIX F2000", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 2048, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0405"}},
		{ProductCode: "A1780P", DisplayName: "SOLIX F2000 Plus", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 2048, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0405"}},
		{ProductCode: "A1782", DisplayName: "SOLIX F3000", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 3072, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0421", "0900"}},
		{ProductCode: "A1783", DisplayName: "SOLIX C2000 Gen 2", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 2048, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0421", "0900"}},
		{ProductCode: "A1790", DisplayName: "SOLIX F3800", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 3840, DefaultPVInputs: 2, MQTTMessageTypes: []string{"0405"}},
		{ProductCode: "A1790P", DisplayName: "SOLIX F3800 Plus", Family: FamilyPowerStation, Status: SupportEnabled, BatteryCapacityWh: 3840, DefaultPVInputs: 2, MQTTMessageTypes: []string{"0405"}},
		{ProductCode: "A17C0", DisplayName: "Solarbank E1600", Family: FamilySolarbank, Status: SupportEnabled, BatteryCapacityWh: 1600, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0405", "0407", "0408"}},
		{ProductCode: "A17C1", DisplayName: "Solarbank 2 E1600 Pro", Family: FamilySolarbank, Status: SupportEnabled, BatteryCapacityWh: 1600, DefaultPVInputs: 4, MQTTMessageTypes: []string{"0405", "0407", "0408", "040a"}},
		{ProductCode: "A17C2", DisplayName: "Solarbank 2 E1600 AC", Family: FamilySolarbank, Status: SupportEnabled, BatteryCapacityWh: 1600, DefaultPVInputs: 2, MQTTMessageTypes: []string{"0405", "0407", "0408", "040a"}},
		{ProductCode: "A17C3", DisplayName: "Solarbank 2 E1600 Plus", Family: FamilySolarbank, Status: SupportEnabled, BatteryCapacityWh: 1600, DefaultPVInputs: 2, MQTTMessageTypes: []string{"0405", "0407", "0408", "040a"}},
		{ProductCode: "A17C5", DisplayName: "Solarbank 3 E2700 Pro", Family: FamilySolarbank, Status: SupportEnabled, BatteryCapacityWh: 2688, DefaultPVInputs: 4, MQTTMessageTypes: []string{"0405", "0407", "0408", "040a"}},
		{ProductCode: "AE100", DisplayName: "Solarbank Power Dock", Family: FamilySolarbank, Status: SupportEnabled, DefaultPVInputs: 0, MQTTMessageTypes: []string{"0420", "0440", "0500"}},
		{ProductCode: "A17B1", DisplayName: "SOLIX Home Power Panel", Family: FamilyPowerPanel, Status: SupportEnabled, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0500"}},
		{ProductCode: "A17E1", DisplayName: "SOLIX E10", Family: FamilyHomeBackup, Status: SupportEnabled, BatteryCapacityWh: 5120, DefaultPVInputs: 2, MQTTMessageTypes: []string{"0405", "0408", "040a"}},
		{ProductCode: "AX170", DisplayName: "SOLIX E10 Power Dock", Family: FamilyHomeBackup, Status: SupportEnabled, DefaultPVInputs: 1, MQTTMessageTypes: []string{"0405", "0666", "0830"}},
		{ProductCode: "A5101", DisplayName: "SOLIX X1 P6K US", Family: FamilyHES, Status: SupportEnabled, DefaultPVInputs: 1, MQTTMessageTypes: []string{"json"}},
		{ProductCode: "A5102", DisplayName: "SOLIX X1 H single phase", Family: FamilyHES, Status: SupportEnabled, DefaultPVInputs: 1, MQTTMessageTypes: []string{"json"}},
		{ProductCode: "A5103", DisplayName: "SOLIX X1 H three phase", Family: FamilyHES, Status: SupportEnabled, DefaultPVInputs: 1, MQTTMessageTypes: []string{"json"}},
		{ProductCode: "A7320", DisplayName: "SOLIX Smart Generator 5500", Family: FamilyGenerator, Status: SupportCompanion, MQTTMessageTypes: []string{"0405", "0408"}},
	} {
		register(capability)
	}
	for _, code := range []string{"A1753", "A1754", "A1755", "A1762", "A1765", "AS100", "A1771", "A1772", "A1781", "A1785", "A17E2"} {
		register(ModelCapability{ProductCode: code, Status: SupportNeedsSample})
	}
	for _, code := range []string{"A17X7", "A17X8", "A17X9", "A5191", "A5143", "A2345", "A17A0", "A17A1", "A17A2", "A17A3", "A17A4", "A17A5"} {
		register(ModelCapability{ProductCode: code, Family: FamilyExcluded, Status: SupportExcluded})
	}
}
