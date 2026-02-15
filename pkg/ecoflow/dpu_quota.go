package ecoflow

import (
	"context"
	"fmt"
	"math"
	"strconv"
	"strings"
)

const (
	dpuAppShowPrefix = "hs_yj751_pd_appshow_addr."
	dpuAppSetPrefix  = "hs_yj751_pd_app_set_info_addr."
	dpuBackendPrefix = "hs_yj751_pd_backend_addr."
	dpuBPAddrPrefix  = "hs_yj751_pd_bp_addr."
	dpuBMSSlavePref  = "hs_yj751_bms_slave_addr."
)

// DPUQuota is a strongly typed view of DELTA Pro Ultra get-all-quota data.
//
// Raw includes all original key/value entries so unmapped fields remain
// available without schema loss.
type DPUQuota struct {
	AppShow DPUAppShowQuota
	AppSet  DPUAppSetQuota
	Backend DPUBackendQuota

	// BatteryPackInfo is parsed from hs_yj751_pd_bp_addr.bpInfo.
	BatteryPackInfo []BatteryPackInfo
	// BatteryPackInfoUpdatedAt is hs_yj751_pd_bp_addr.updateTime.
	BatteryPackInfoUpdatedAt string

	// KitInfoWatts is parsed from bms_kitInfo.watts when present.
	KitInfoWatts []KitInfoWattsEntry

	// BMSSlaves contains per-pack status, keyed by pack index from
	// hs_yj751_bms_slave_addr.<index>.*.
	BMSSlaves map[int]DPUBMSSlaveQuota

	Raw map[string]string
}

// DPUAppShowQuota contains frequently-used app display values.
type DPUAppShowQuota struct {
	Access5p8InType  int64
	Access5p8OutType int64
	AccessType       int64
	BPNum            int64
	C20ChgMaxWatts   float64

	ChgTimeTaskAddParam int64
	ChgTimeTaskIndex    int64
	ChgTimeTaskMode     int64
	ChgTimeTaskNotice   int64
	ChgTimeTaskParam    int64
	ChgTimeTaskTable0   int64
	ChgTimeTaskTable1   int64
	ChgTimeTaskTable2   int64
	ChgTimeTaskType     int64

	DsgTimeTaskAddParam int64
	DsgTimeTaskIndex    int64
	DsgTimeTaskMode     int64
	DsgTimeTaskNotice   int64
	DsgTimeTaskParam    int64
	DsgTimeTaskTable0   int64
	DsgTimeTaskTable1   int64
	DsgTimeTaskTable2   int64
	DsgTimeTaskType     int64

	FullCombo int64
	// InAc5p8Pwr is AC 5.8k input power.
	InAc5p8Pwr  float64
	InAcC20Pwr  float64
	InHvMpptPwr float64
	InLvMpptPwr float64
	OutAc5p8Pwr float64
	OutAcL11Pwr float64
	OutAcL12Pwr float64
	OutAcL14Pwr float64
	OutAcL21Pwr float64
	OutAcL22Pwr float64
	// OutAcTtPwr is AC30A output power.
	OutAcTtPwr      float64
	OutAdsPwr       float64
	OutPrPwr        float64
	OutTypec1Pwr    float64
	OutTypec2Pwr    float64
	OutUsb1Pwr      float64
	OutUsb2Pwr      float64
	ParaChgMaxWatts float64
	PCSType         int64
	ProtoVer        int64
	RemainCombo     int64
	// WattsInSum is total input power.
	WattsInSum float64
	// WattsOutSum is total output power.
	WattsOutSum float64
	// RemainTime is the estimated remaining runtime (seconds/minutes per API).
	RemainTime int64
	// Soc is state of charge shown in app.
	Soc                  int64
	ShowFlag             int64
	SimICCID             string
	SysErrCode           int64
	TimeTaskChangeCnt    int64
	TimeTaskConflictFlag int64
	UpdateTime           string
	Wireless4GSta        int64
	Wireless4GCon        int64
	Wireless4GOn         int64
	Wireless4GErrCode    int64

	Raw map[string]string
}

// DPUAppSetQuota contains selected writable policy/config values.
type DPUAppSetQuota struct {
	AcInBanFlag       int64
	AcOftenOpenFlg    int64
	AcOftenOpenMinSoc int64
	AcOutFreq         int64
	AcStandbyMins     int64
	AcXboost          int64
	BackupRatio       int64
	BMSModeSet        int64
	BypassDsgEnFlag   int64
	Chg5p8SetWatts    float64
	ChgC20SetWatts    float64
	// ChgMaxSoc is the configured max SOC.
	ChgMaxSoc      int64
	ChgPfcSetWatts float64
	DcStandbyMins  int64
	// DsgMinSoc is the configured min SOC.
	DsgMinSoc          int64
	EnergyManageEnable int64
	PowerStandbyMins   int64
	ScreenStandbySec   int64
	SolarOnlyFlg       int64
	SysBackupEvent     int64
	SysBackupSoc       int64
	// SysTimezone is configured timezone offset minutes.
	SysTimezone int64
	// SysTimezoneID is configured timezone identifier.
	SysTimezoneID   string
	SysWordMode     int64
	TimezoneSetType int64
	// UpdateTime is last update timestamp.
	UpdateTime string

	Raw map[string]string
}

// DPUBackendQuota contains selected backend runtime telemetry.
type DPUBackendQuota struct {
	AcInFreq   float64
	AcOutFreq  float64
	AdsErrCode int64
	// BatAmp is battery current.
	BatAmp float64
	// BatVol is battery voltage.
	BatVol                           float64
	BMSHealthPropertyUploadPeriod    int64
	BMSHeartbeatPropertyUploadPeriod int64
	// BMSInputWatts is BMS input power.
	BMSInputWatts float64
	// BMSOutputWatts is BMS output power.
	BMSOutputWatts                         float64
	BMSInfoPropertyFullUploadPeriod        int64
	C20InType                              int64
	ChgReignSta                            int64
	DisplayPropertyFullUploadPeriod        int64
	DisplayPropertyIncrementalUploadPeriod int64
	EMSMaxAvailNum                         int64
	EMSOpenBmsIdx                          int64
	EMSParaVolMax                          float64
	EMSParaVolMin                          float64
	EMSWorkSta                             int64
	EVMaxChargerCur                        float64
	FanState                               int64
	HVPvErrCode                            int64
	InAc5p8Amp                             float64
	InAc5p8Vol                             float64
	InAcC20Amp                             float64
	InAcC20Vol                             float64
	InHvMpptAmp                            float64
	InHvMpptVol                            float64
	InLvMpptAmp                            float64
	InLvMpptVol                            float64
	LVPvErrCode                            int64
	MpptHvTemp                             int64
	MpptLvTemp                             int64
	OutAc5p8Amp                            float64
	OutAc5p8Vol                            float64
	OutAcL11Amp                            float64
	OutAcL11Pf                             float64
	OutAcL11Vol                            float64
	OutAcL12Amp                            float64
	OutAcL12Pf                             float64
	OutAcL12Vol                            float64
	OutAcL14Amp                            float64
	OutAcL14Pf                             float64
	OutAcL14Vol                            float64
	OutAcL21Amp                            float64
	OutAcL21Pf                             float64
	OutAcL21Vol                            float64
	OutAcL22Amp                            float64
	OutAcL22Pf                             float64
	OutAcL22Vol                            float64
	OutAcP58Pf                             float64
	OutAcTtAmp                             float64
	OutAcTtPf                              float64
	OutAcTtVol                             float64
	OutAdsAmp                              float64
	OutAdsVol                              float64
	OutTypec1Amp                           float64
	OutTypec1Vol                           float64
	OutTypec2Amp                           float64
	OutTypec2Vol                           float64
	OutUsb1Amp                             float64
	OutUsb1Vol                             float64
	OutUsb2Amp                             float64
	OutUsb2Vol                             float64
	PCSAcErrCode                           int64
	PCSAcTemp                              int64
	PCSDcErrCode                           int64
	PCSDcTemp                              int64
	PCSWorkSta                             int64
	PDTemp                                 int64
	RecordFlag                             int64
	RuntimePropertyFullUploadPeriod        int64
	RuntimePropertyIncrementalUploadPeriod int64
	// SysWorkSta is system work status code.
	SysWorkSta int64
	// UpdateTime is backend telemetry timestamp.
	UpdateTime  string
	Work5p8Mode int64

	Raw map[string]string
}

// DPUBMSSlaveQuota contains typed BMS slave values.
type DPUBMSSlaveQuota struct {
	Index int

	AccuChgCap int64
	AccuDsgCap int64
	PackSN     string
	Soc        int64
	Soh        int64
	ActSoc     int64
	ActSoh     float64
	AdBatVol   float64

	AplVer                                int64
	BalanceState                          int64
	BMSFault                              int64
	BPDiffSoc                             int64
	BPLimitSoc                            int64
	BqSysStatReg                          int64
	CellID                                int64
	ChgMaxSoc                             int64
	ChgSocProState                        int64
	CurResTemp                            int64
	Cycles                                int64
	DesignCap                             int64
	DsgMinSoc                             int64
	DsgSocProState                        int64
	ErrCode                               int64
	F32ShowSoc                            float64
	FullCap                               int64
	HWBoardTemp                           int64
	HWVer                                 int64
	InstallmentPaymentOverdueLimit        string
	InstallmentPaymentOverdueLimitUTCTime int64
	InstallmentPaymentServeEnable         bool
	InstallmentPaymentStartUTCTime        int64
	InstallmentPaymentState               string
	LoaderSwVer                           int64
	MaxCellTemp                           int64
	MaxCellVol                            int64
	MaxMosTemp                            int64
	MaxVolDiff                            int64
	MinCellTemp                           int64
	MinCellVol                            int64
	MinMosTemp                            int64
	MosState                              int64
	Num                                   int64
	OCV                                   int64
	OpenBmsFlag                           int64
	PackHeartbeatVer                      int64
	ProductDetail                         int64
	ProductType                           int64
	PtcHeatingEvent                       int64
	PtcMosState                           int64
	PtcTouchWay                           int64
	RemainCap                             int64
	RemainTime                            int64
	ServeMiddlemen                        string
	TagChgAmp                             int64
	TargetSoc                             float64
	Type                                  int64
	UnixTime                              int64

	InputWatts  float64
	OutputWatts float64
	Temp        int64
	Vol         float64
	Amp         float64

	CellTemp    []uint64
	CellVol     []uint64
	AllErrFlag  []uint64
	MosTemp     []uint64
	PtcTemp     []uint64
	IcoBytes    []uint64
	Reserved    []uint64
	HWVersion   []uint64
	BMSIsConnt  []uint64
	BMSKitState []uint64

	Raw map[string]string
}

// GetDPUQuota fetches get-all-quota values and maps known DELTA Pro Ultra
// fields into a typed DPUQuota structure.
func (s *GeneralInfoService) GetDPUQuota(
	ctx context.Context,
	sn string,
) (DPUQuota, Response, error) {
	values, response, err := s.GetDeviceAllQuota(ctx, sn)
	if err != nil {
		return DPUQuota{}, response, err
	}

	typed, err := ParseDPUQuota(values)
	if err != nil {
		return DPUQuota{}, response, err
	}
	return typed, response, nil
}

// ParseDPUQuota converts raw get-all-quota values into typed DELTA Pro Ultra
// sections while preserving raw values.
func ParseDPUQuota(values map[string]string) (DPUQuota, error) {
	quota := DPUQuota{
		AppShow: DPUAppShowQuota{
			Raw: make(map[string]string),
		},
		AppSet: DPUAppSetQuota{
			Raw: make(map[string]string),
		},
		Backend: DPUBackendQuota{
			Raw: make(map[string]string),
		},
		BMSSlaves: make(map[int]DPUBMSSlaveQuota),
		Raw:       cloneStringMap(values),
	}

	parseErrs := make([]string, 0, 4)
	setErr := func(key string, err error) {
		if err == nil {
			return
		}
		parseErrs = append(parseErrs, fmt.Sprintf("%s: %v", key, err))
	}

	for key, value := range values {
		switch {
		case strings.HasPrefix(key, dpuAppShowPrefix):
			suffix := strings.TrimPrefix(key, dpuAppShowPrefix)
			quota.AppShow.Raw[suffix] = value
			switch suffix {
			case "access5p8InType":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.Access5p8InType = parsed
			case "access5p8OutType":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.Access5p8OutType = parsed
			case "accessType":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.AccessType = parsed
			case "bpNum":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.BPNum = parsed
			case "c20ChgMaxWatts":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.C20ChgMaxWatts = parsed
			case "chgTimeTaskAddparam":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ChgTimeTaskAddParam = parsed
			case "chgTimeTaskIndex":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ChgTimeTaskIndex = parsed
			case "chgTimeTaskMode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ChgTimeTaskMode = parsed
			case "chgTimeTaskNotice":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ChgTimeTaskNotice = parsed
			case "chgTimeTaskParam":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ChgTimeTaskParam = parsed
			case "chgTimeTaskTable0":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ChgTimeTaskTable0 = parsed
			case "chgTimeTaskTable1":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ChgTimeTaskTable1 = parsed
			case "chgTimeTaskTable2":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ChgTimeTaskTable2 = parsed
			case "chgTimeTaskType":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ChgTimeTaskType = parsed
			case "dsgTimeTaskAddparam":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.DsgTimeTaskAddParam = parsed
			case "dsgTimeTaskIndex":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.DsgTimeTaskIndex = parsed
			case "dsgTimeTaskMode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.DsgTimeTaskMode = parsed
			case "dsgTimeTaskNotice":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.DsgTimeTaskNotice = parsed
			case "dsgTimeTaskParam":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.DsgTimeTaskParam = parsed
			case "dsgTimeTaskTable0":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.DsgTimeTaskTable0 = parsed
			case "dsgTimeTaskTable1":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.DsgTimeTaskTable1 = parsed
			case "dsgTimeTaskTable2":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.DsgTimeTaskTable2 = parsed
			case "dsgTimeTaskType":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.DsgTimeTaskType = parsed
			case "fullCombo":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.FullCombo = parsed
			case "inAc5p8Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.InAc5p8Pwr = parsed
			case "inAcC20Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.InAcC20Pwr = parsed
			case "inHvMpptPwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.InHvMpptPwr = parsed
			case "inLvMpptPwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.InLvMpptPwr = parsed
			case "outAc5p8Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutAc5p8Pwr = parsed
			case "outAcL11Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutAcL11Pwr = parsed
			case "outAcL12Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutAcL12Pwr = parsed
			case "outAcL14Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutAcL14Pwr = parsed
			case "outAcL21Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutAcL21Pwr = parsed
			case "outAcL22Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutAcL22Pwr = parsed
			case "outAcTtPwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutAcTtPwr = parsed
			case "outAdsPwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutAdsPwr = parsed
			case "outPrPwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutPrPwr = parsed
			case "outTypec1Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutTypec1Pwr = parsed
			case "outTypec2Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutTypec2Pwr = parsed
			case "outUsb1Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutUsb1Pwr = parsed
			case "outUsb2Pwr":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.OutUsb2Pwr = parsed
			case "paraChgMaxWatts":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.ParaChgMaxWatts = parsed
			case "pcsType":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.PCSType = parsed
			case "protoVer":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ProtoVer = parsed
			case "remainCombo":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.RemainCombo = parsed
			case "wattsInSum":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.WattsInSum = parsed
			case "wattsOutSum":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppShow.WattsOutSum = parsed
			case "remainTime":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.RemainTime = parsed
			case "soc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.Soc = parsed
			case "showFlag":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.ShowFlag = parsed
			case "simIccid":
				quota.AppShow.SimICCID = value
			case "sysErrCode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.SysErrCode = parsed
			case "timeTaskChangeCnt":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.TimeTaskChangeCnt = parsed
			case "timeTaskConflictFlag":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.TimeTaskConflictFlag = parsed
			case "updateTime":
				quota.AppShow.UpdateTime = value
			case "wireless4GSta":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.Wireless4GSta = parsed
			case "wireless4gCon":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.Wireless4GCon = parsed
			case "wireless4gOn":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.Wireless4GOn = parsed
			case "wirlesss4gErrCode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppShow.Wireless4GErrCode = parsed
			}

		case strings.HasPrefix(key, dpuAppSetPrefix):
			suffix := strings.TrimPrefix(key, dpuAppSetPrefix)
			quota.AppSet.Raw[suffix] = value
			switch suffix {
			case "acInBanFlag":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.AcInBanFlag = parsed
			case "acOftenOpenFlg":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.AcOftenOpenFlg = parsed
			case "acOftenOpenMinSoc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.AcOftenOpenMinSoc = parsed
			case "acOutFreq":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.AcOutFreq = parsed
			case "acStandbyMins":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.AcStandbyMins = parsed
			case "acXboost":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.AcXboost = parsed
			case "backupRatio":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.BackupRatio = parsed
			case "bmsModeSet":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.BMSModeSet = parsed
			case "bypassDsgEnFlag":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.BypassDsgEnFlag = parsed
			case "chg5p8SetWatts":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppSet.Chg5p8SetWatts = parsed
			case "chgC20SetWatts":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppSet.ChgC20SetWatts = parsed
			case "chgMaxSoc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.ChgMaxSoc = parsed
			case "chgPfcSetWatts":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.AppSet.ChgPfcSetWatts = parsed
			case "dcStandbyMins":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.DcStandbyMins = parsed
			case "dsgMinSoc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.DsgMinSoc = parsed
			case "energyMamageEnable":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.EnergyManageEnable = parsed
			case "powerStandbyMins":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.PowerStandbyMins = parsed
			case "screenStandbySec":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.ScreenStandbySec = parsed
			case "solarOnlyFlg":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.SolarOnlyFlg = parsed
			case "sysBackupEvent":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.SysBackupEvent = parsed
			case "sysBackupSoc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.SysBackupSoc = parsed
			case "sysTimezone":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.SysTimezone = parsed
			case "sysTimezoneId":
				quota.AppSet.SysTimezoneID = value
			case "sysWordMode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.SysWordMode = parsed
			case "timezoneSettype":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.AppSet.TimezoneSetType = parsed
			case "updateTime":
				quota.AppSet.UpdateTime = value
			}

		case strings.HasPrefix(key, dpuBackendPrefix):
			suffix := strings.TrimPrefix(key, dpuBackendPrefix)
			quota.Backend.Raw[suffix] = value
			switch suffix {
			case "acInFreq":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.AcInFreq = parsed
			case "acOutFreq":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.AcOutFreq = parsed
			case "adsErrCode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.AdsErrCode = parsed
			case "batAmp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.BatAmp = parsed
			case "batVol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.BatVol = parsed
			case "bmsInputWatts":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.BMSInputWatts = parsed
			case "bmsOutputWatts":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.BMSOutputWatts = parsed
			case "bmsHealthPropertyUploadPeriod":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.BMSHealthPropertyUploadPeriod = parsed
			case "bmsHeartbeatPropertyUploadPeriod":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.BMSHeartbeatPropertyUploadPeriod = parsed
			case "bmsinfoPropertyFullUploadPeriod":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.BMSInfoPropertyFullUploadPeriod = parsed
			case "c20InType":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.C20InType = parsed
			case "chgReignSta":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.ChgReignSta = parsed
			case "displayPropertyFullUploadPeriod":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.DisplayPropertyFullUploadPeriod = parsed
			case "displayPropertyIncrementalUploadPeriod":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.DisplayPropertyIncrementalUploadPeriod = parsed
			case "emsMaxAvailNum":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.EMSMaxAvailNum = parsed
			case "emsOpenBmsIdx":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.EMSOpenBmsIdx = parsed
			case "emsParaVolMax":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.EMSParaVolMax = parsed
			case "emsParaVolMin":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.EMSParaVolMin = parsed
			case "emsWorkSta":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.EMSWorkSta = parsed
			case "evMaxChargerCur":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.EVMaxChargerCur = parsed
			case "fanState":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.FanState = parsed
			case "hvPvErrCode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.HVPvErrCode = parsed
			case "inAc5p8Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.InAc5p8Amp = parsed
			case "inAc5p8Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.InAc5p8Vol = parsed
			case "inAcC20Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.InAcC20Amp = parsed
			case "inAcC20Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.InAcC20Vol = parsed
			case "inHvMpptAmp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.InHvMpptAmp = parsed
			case "inHvMpptVol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.InHvMpptVol = parsed
			case "inLvMpptAmp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.InLvMpptAmp = parsed
			case "inLvMpptVol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.InLvMpptVol = parsed
			case "lvPvErrCode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.LVPvErrCode = parsed
			case "mpptHvTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.MpptHvTemp = parsed
			case "mpptLvTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.MpptLvTemp = parsed
			case "outAc5p8Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAc5p8Amp = parsed
			case "outAc5p8Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAc5p8Vol = parsed
			case "outAcL11Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL11Amp = parsed
			case "outAcL11Pf":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL11Pf = parsed
			case "outAcL11Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL11Vol = parsed
			case "outAcL12Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL12Amp = parsed
			case "outAcL12Pf":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL12Pf = parsed
			case "outAcL12Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL12Vol = parsed
			case "outAcL14Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL14Amp = parsed
			case "outAcL14Pf":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL14Pf = parsed
			case "outAcL14Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL14Vol = parsed
			case "outAcL21Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL21Amp = parsed
			case "outAcL21Pf":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL21Pf = parsed
			case "outAcL21Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL21Vol = parsed
			case "outAcL22Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL22Amp = parsed
			case "outAcL22Pf":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL22Pf = parsed
			case "outAcL22Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcL22Vol = parsed
			case "outAcP58Pf":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcP58Pf = parsed
			case "outAcTtAmp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcTtAmp = parsed
			case "outAcTtPf":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcTtPf = parsed
			case "outAcTtVol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAcTtVol = parsed
			case "outAdsAmp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAdsAmp = parsed
			case "outAdsVol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutAdsVol = parsed
			case "outTypec1Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutTypec1Amp = parsed
			case "outTypec1Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutTypec1Vol = parsed
			case "outTypec2Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutTypec2Amp = parsed
			case "outTypec2Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutTypec2Vol = parsed
			case "outUsb1Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutUsb1Amp = parsed
			case "outUsb1Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutUsb1Vol = parsed
			case "outUsb2Amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutUsb2Amp = parsed
			case "outUsb2Vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				quota.Backend.OutUsb2Vol = parsed
			case "pcsAcErrCode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.PCSAcErrCode = parsed
			case "pcsAcTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.PCSAcTemp = parsed
			case "pcsDcErrCode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.PCSDcErrCode = parsed
			case "pcsDcTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.PCSDcTemp = parsed
			case "pcsWorkSta":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.PCSWorkSta = parsed
			case "pdTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.PDTemp = parsed
			case "recordFlag":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.RecordFlag = parsed
			case "runtimePropertyFullUploadPeriod":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.RuntimePropertyFullUploadPeriod = parsed
			case "runtimePropertyIncrementalUploadPeriod":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.RuntimePropertyIncrementalUploadPeriod = parsed
			case "sysWorkSta":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.SysWorkSta = parsed
			case "updateTime":
				quota.Backend.UpdateTime = value
			case "work5p8Mode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				quota.Backend.Work5p8Mode = parsed
			}

		case strings.HasPrefix(key, dpuBPAddrPrefix):
			suffix := strings.TrimPrefix(key, dpuBPAddrPrefix)
			switch suffix {
			case "bpInfo":
				parsed, err := ParseBatteryPackInfo(value)
				setErr(key, err)
				quota.BatteryPackInfo = parsed
			case "updateTime":
				quota.BatteryPackInfoUpdatedAt = value
			}

		case strings.HasPrefix(key, dpuBMSSlavePref):
			index, field, ok := parseIndexedDPUField(key, dpuBMSSlavePref)
			if !ok {
				continue
			}
			slave := quota.BMSSlaves[index]
			slave.Index = index
			if slave.Raw == nil {
				slave.Raw = make(map[string]string)
			}
			slave.Raw[field] = value

			switch field {
			case "accuChgCap":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.AccuChgCap = parsed
			case "accuDsgCap":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.AccuDsgCap = parsed
			case "packSn":
				slave.PackSN = value
			case "soc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.Soc = parsed
			case "soh":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.Soh = parsed
			case "actSoc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.ActSoc = parsed
			case "actSoh":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				slave.ActSoh = parsed
			case "adBatVol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				slave.AdBatVol = parsed
			case "aplVer":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.AplVer = parsed
			case "balanceState":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.BalanceState = parsed
			case "bmsFault":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.BMSFault = parsed
			case "bpDiffSoc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.BPDiffSoc = parsed
			case "bpLimitSoc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.BPLimitSoc = parsed
			case "bqSysStatReg":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.BqSysStatReg = parsed
			case "cellId":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.CellID = parsed
			case "chgMaxSoc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.ChgMaxSoc = parsed
			case "chgSocProState":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.ChgSocProState = parsed
			case "curResTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.CurResTemp = parsed
			case "cycles":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.Cycles = parsed
			case "designCap":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.DesignCap = parsed
			case "dsgMinSoc":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.DsgMinSoc = parsed
			case "dsgSocProState":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.DsgSocProState = parsed
			case "errCode":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.ErrCode = parsed
			case "f32ShowSoc":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				slave.F32ShowSoc = parsed
			case "fullCap":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.FullCap = parsed
			case "hwBoardTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.HWBoardTemp = parsed
			case "hwVer":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.HWVer = parsed
			case "installmentPaymentOverdueLimit":
				slave.InstallmentPaymentOverdueLimit = value
			case "installmentPaymentOverdueLimitUtcTime":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.InstallmentPaymentOverdueLimitUTCTime = parsed
			case "installmentPaymentServeEnable":
				parsed, err := parseBoolQuota(value)
				setErr(key, err)
				slave.InstallmentPaymentServeEnable = parsed
			case "installmentPaymentStartUtcTime":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.InstallmentPaymentStartUTCTime = parsed
			case "installmentPaymentState":
				slave.InstallmentPaymentState = value
			case "loaderSwVer":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.LoaderSwVer = parsed
			case "maxCellTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.MaxCellTemp = parsed
			case "maxCellVol":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.MaxCellVol = parsed
			case "maxMosTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.MaxMosTemp = parsed
			case "maxVolDiff":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.MaxVolDiff = parsed
			case "minCellTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.MinCellTemp = parsed
			case "minCellVol":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.MinCellVol = parsed
			case "minMosTemp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.MinMosTemp = parsed
			case "mosState":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.MosState = parsed
			case "num":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.Num = parsed
			case "ocv":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.OCV = parsed
			case "openBmsFlag":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.OpenBmsFlag = parsed
			case "packHeartbeatVer":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.PackHeartbeatVer = parsed
			case "productDetail":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.ProductDetail = parsed
			case "productType":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.ProductType = parsed
			case "ptcHeatingEvent":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.PtcHeatingEvent = parsed
			case "ptcMosState":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.PtcMosState = parsed
			case "ptcTouchWay":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.PtcTouchWay = parsed
			case "remainCap":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.RemainCap = parsed
			case "remainTime":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.RemainTime = parsed
			case "serveMiddlemen":
				slave.ServeMiddlemen = value
			case "tagChgAmp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.TagChgAmp = parsed
			case "targetSoc":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				slave.TargetSoc = parsed
			case "type":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.Type = parsed
			case "unixTime":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.UnixTime = parsed
			case "inputWatts":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				slave.InputWatts = parsed
			case "outputWatts":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				slave.OutputWatts = parsed
			case "temp":
				parsed, err := parseInt64Quota(value)
				setErr(key, err)
				slave.Temp = parsed
			case "vol":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				slave.Vol = parsed
			case "amp":
				parsed, err := parseFloat64Quota(value)
				setErr(key, err)
				slave.Amp = parsed
			case "cellTemp":
				parsed, err := ParseUnsignedIntArray(value)
				setErr(key, err)
				slave.CellTemp = parsed
			case "cellVol":
				parsed, err := ParseUnsignedIntArray(value)
				setErr(key, err)
				slave.CellVol = parsed
			case "allErrFlag":
				parsed, err := ParseUnsignedIntArray(value)
				setErr(key, err)
				slave.AllErrFlag = parsed
			case "mosTemp":
				parsed, err := ParseUnsignedIntArray(value)
				setErr(key, err)
				slave.MosTemp = parsed
			case "ptcTemp":
				parsed, err := ParseUnsignedIntArray(value)
				setErr(key, err)
				slave.PtcTemp = parsed
			case "icoBytes":
				parsed, err := ParseUnsignedIntArray(value)
				setErr(key, err)
				slave.IcoBytes = parsed
			case "reserved":
				parsed, err := ParseUnsignedIntArray(value)
				setErr(key, err)
				slave.Reserved = parsed
			case "hwVersion":
				parsed, err := ParseUnsignedIntArray(value)
				setErr(key, err)
				slave.HWVersion = parsed
			case "bmsIsConnt":
				parsed, err := ParseUnsignedIntArray(value)
				setErr(key, err)
				slave.BMSIsConnt = parsed
			case "bmsKitState":
				parsed, err := ParseUnsignedIntArray(value)
				setErr(key, err)
				slave.BMSKitState = parsed
			}
			quota.BMSSlaves[index] = slave
		}

		if strings.HasSuffix(key, "kitInfo.watts") {
			parsed, err := ParseKitInfoWatts(value)
			setErr(key, err)
			quota.KitInfoWatts = parsed
		}
	}

	if len(parseErrs) > 0 {
		return quota, fmt.Errorf("parse dpu quota: %s", strings.Join(parseErrs, "; "))
	}
	return quota, nil
}

func parseIndexedDPUField(key string, prefix string) (int, string, bool) {
	rest := strings.TrimPrefix(key, prefix)
	if rest == key {
		return 0, "", false
	}
	parts := strings.SplitN(rest, ".", 2)
	if len(parts) != 2 {
		return 0, "", false
	}
	index, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, "", false
	}
	return index, parts[1], true
}

func parseInt64Quota(value string) (int64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err == nil {
		return parsed, nil
	}
	asFloat, floatErr := strconv.ParseFloat(value, 64)
	if floatErr != nil || math.Trunc(asFloat) != asFloat {
		return 0, fmt.Errorf("parse int64 %q: %w", value, err)
	}
	return int64(asFloat), nil
}

func parseFloat64Quota(value string) (float64, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, nil
	}
	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return 0, fmt.Errorf("parse float64 %q: %w", value, err)
	}
	return parsed, nil
}

func parseBoolQuota(value string) (bool, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return false, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err == nil {
		return parsed, nil
	}
	asInt, intErr := strconv.ParseInt(value, 10, 64)
	if intErr == nil {
		return asInt != 0, nil
	}
	return false, fmt.Errorf("parse bool %q: %w", value, err)
}

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return map[string]string{}
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
