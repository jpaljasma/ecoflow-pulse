
# Config 05 — EcoFlow Device Quota API Response Fields

This enables realtime metadata and telemetry mapping.

---

High-level shape:
- `Delta 2`: single-PV, `bmsMaster` + `bmsSlave1` + `ems`
- `Delta Pro 3`: 112 fields
- `Delta Pro Ultra`: 91 fields
- `Delta 2 Max`: 187 fields

I would group them like this.

**1. Device State And Errors**
Common meaning:
- on/off, sleep, standby, run state, error code, warning/fault state

Examples:
- `Delta Pro 3`: `errcode`, `devSleepState`, `devStandbyTime`, `acStandbyTime`, `dcStandbyTime`
- `Delta Pro Ultra`: `...appshow_addr.sysErrCode`, `...backend_addr.LVPvErrCode`, `...backend_addr.HVPvErrCode`, `...backend_addr.PCSAcErrCode`
- `Delta 2 Max`: `pd.chgDsgState`, `bms_bmsStatus.errCode`, `bms_bmsStatus.bmsFault`, `mppt.faultCode`

Store target:
- live telemetry: yes
- metadata: yes

**2. SOC, ETA, Limits, Reserve**
Common meaning:
- SOC, charge/discharge remaining time, min/max SOC, backup reserve

Examples:
- `Delta Pro 3`: `cmsBattSoc`, `cmsChgRemTime`, `cmsDsgRemTime`, `cmsMinDsgSoc`, `cmsMaxChgSoc`
- `Delta Pro Ultra`: `...appshow_addr.soc`, `...appshow_addr.remainTime`, `...appset_addr.chgMaxSoc`, `...appset_addr.dsgMinSoc`, `...appset_addr.backupRatio`, `...appset_addr.sysBackupSoc`
- `Delta 2 Max`: `pd.soc`, `pd.remainTime`, `bms_emsStatus.maxChargeSoc`, `pd.bpPowerSoc`, `pd.minAcSoc`

Store target:
- live telemetry: yes
- rollups/history: yes
- metadata: reserve/limits also in metadata

**3. Battery Electrical Telemetry**
Common meaning:
- battery voltage, current, input watts, output watts, capacity

Examples:
- `Delta Pro 3`: `bmsDesignCap`, `bmsBattSoc`, `bmsChgDsgState`
- `Delta Pro Ultra`: `...backend_addr.batVol`, `...backend_addr.batAmp`, `...backend_addr.BMSInputWatts`, `...backend_addr.BMSOutputWatts`
- `Delta 2 Max`: `bms_bmsStatus.vol`, `bms_bmsStatus.inputWatts`, `bms_bmsStatus.outputWatts`, `bms_bmsStatus.designCap`, `bms_bmsStatus.fullCap`, `bms_bmsStatus.remainCap`

Store target:
- live telemetry: yes
- rollups/history: yes

**4. Battery Thermal / Heating / Fan**
Common meaning:
- battery temp, pack temp, MOS temp, fan level, heating/preconditioning

Examples:
- `Delta Pro 3`: `bmsMaxCellTemp`, `bmsMinCellTemp`
- `Delta Pro Ultra`: `...backend_addr.PDTemp`, `...backend_addr.MpptHvTemp`, `...backend_addr.MpptLvTemp`, `...backend_addr.FanState`, `...appset_addr.bmsModeSet`
- `Delta 2 Max`: `bms_bmsStatus.temp`, `bms_bmsStatus.cellTemp`, `bms_bmsStatus.maxCellTemp`, `bms_bmsStatus.minMosTemp`, `inv.fanState`, `bms_emsStatus.fanLevel`

Store target:
- live telemetry: yes
- metadata: heating capability flag

**5. Aggregate Power**
Common meaning:
- total input power, total output power

Examples:
- `Delta Pro 3`: `powInSumW`, `powOutSumW`
- `Delta Pro Ultra`: `...appshow_addr.wattsInSum`, `...appshow_addr.wattsOutSum`
- `Delta 2 Max`: `pd.wattsInSum`, `pd.wattsOutSum`

Store target:
- live telemetry: yes
- rollups/history: yes

**6. Per-Port Output / Load Power**
Common meaning:
- AC, USB, Type-C, 12V, 24V, Anderson, parallel, extra battery port power

Examples:
- `Delta Pro 3`: `powGetAc`, `powGetAcHvOut`, `powGetTypec1`, `powGetTypec2`, `powGet12v`, `powGet24v`, `powGet5p8`, `powGet4p81`, `powGet4p82`
- `Delta Pro Ultra`: `...appshow_addr.outAcTtPwr`, `outAcL11Pwr`, `outAcL12Pwr`, `outAcL21Pwr`, `outAcL22Pwr`, `outAcL14Pwr`, `outAc_5p8Pwr`, `outAdsPwr`, `outUsb1Pwr`, `outUsb2Pwr`, `outTypec1Pwr`, `outTypec2Pwr`
- `Delta 2 Max`: `inv.outputWatts`, `pd.typec1Watts`, `pd.typec2Watts`, `pd.qcUsb2Watts`, `pd.wireWatts`

Store target:
- live telemetry: yes
- rollups/history: yes

**7. AC Input / Output Electricals**
Common meaning:
- AC in/out volts, amps, freq, charging power

Examples:
- `Delta Pro 3`: `powGetAcIn`, `plugInInfoAcInChgPowMax`, `plugInInfoAcInFeq`, `acOutFreq`
- `Delta Pro Ultra`: `...backend_addr.inAc5p8Vol`, `inAc5p8Amp`, `inAcC20Vol`, `inAcC20Amp`, `outAc*Vol`, `outAc*Amp`, `outAc*Pf`, `...appset_addr.acOutFreq`
- `Delta 2 Max`: `inv.inputWatts`, `inv.acInVol`, `inv.acInAmp`, `inv.acInFreq`, `inv.invOutVol`, `inv.invOutAmp`, `inv.invOutFreq`, `inv.acChgRatedPower`

Store target:
- live telemetry: yes
- capabilities: max AC charge power
- metadata: AC mode/settings

**8. Solar / MPPT / DC Input**
Common meaning:
- PV watts, volts, amps, charge state/type, configured charge source

Examples:
- `Delta 2`: `mppt.inVol`, `mppt.inAmp`, `mppt.outWatts`, `pd.bpPowerSoc`, `pd.minAcSoc`, `ems.maxChargeSoc`, `bmsMaster.fullCap`
- `Delta Pro 3`: `powGetPvH`, `powGetPvL`, `plugInInfoPvH*`, `plugInInfoPvL*`, `flowInfoPvH`, `flowInfoPvL`
- `Delta Pro Ultra`: `...appshow_addr.inHvMpptPwr`, `inLvMpptPwr`, `...backend_addr.inHvMpptVol`, `inHvMpptAmp`, `inLvMpptVol`, `inLvMpptAmp`
- `Delta 2 Max`: `mppt.inVol`, `mppt.inAmp`, `mppt.outWatts`, `mppt.outVol`, `mppt.outAmp`, `mppt.pv2InVol`, `mppt.pv2InAmp`, `pd.pv1ChargeWatts`, `pd.pv1ChargeType`, `pd.pv2ChargeType`, `mppt.chgState`, `mppt.pv2ChgState`, `mppt.chgType`, `mppt.pv2ChgType`

Store target:
- live telemetry: yes
- rollups/history: yes
- capabilities: PV port count and max volts/amps/watts

**9. Port Connectivity / Topology**
Common meaning:
- whether ports are connected, charger presence, what is attached, SNs of attached ecosystem devices

Examples:
- `Delta Pro 3`: all `plugInInfo*` fields, especially `...Flag`, `...ChargerFlag`, `...RunState`, `...Sn`
- `Delta Pro Ultra`: `bpNum`, `access5p8InType`, `access5p8OutType`
- `Delta 2 Max`: `bms_emsStatus.openBmsIdx`, `pd.otherKitState`, `pd.model`, `pd.XT150Watts1`, `pd.XT150Watts2`

Store target:
- metadata: yes
- capabilities: yes

**10. Switch States / Flow States**
Common meaning:
- whether AC/DC/USB/PV paths are enabled or active

Examples:
- `Delta Pro 3`: all `flowInfo*`
- `Delta Pro Ultra`: many of these are inferred from `appshow/backend` instead of explicit `flowInfo`
- `Delta 2 Max`: `inv.cfgAcEnabled`, `pd.carState`, `mppt.carState`, `mppt.dc24vState`

Store target:
- live telemetry: yes
- metadata: sometimes

**11. User Settings / Policy**
Common meaning:
- xboost, AC always-on, screen timeout, standby timers, energy backup, solar priority, quiet mode

Examples:
- `Delta Pro 3`: `acEnergySavingOpen`, `energyBackupEn`, `energyBackupStartSoc`, `xboostEn`, `screenOffTime`, `acLvAlwaysOn`, `acHvAlwaysOn`, `multiBpChgDsgMode`
- `Delta Pro Ultra`: `...appset_addr.acOftenOpenFlg`, `acOftenOpenMinSoc`, `chgMaxSoc`, `dsgMinSoc`, `backupRatio`, `energyMamageEnable`, `screenStandbySec`, `sysWordMode`
- `Delta 2 Max`: `inv.cfgAcWorkMode`, `inv.cfgAcXboost`, `inv.acPassbyAutoEn`, `pd.pvChargePrioSet`, `pd.acAutoOnCfg`, `pd.newAcAutoOnCfg`, `pd.lcdOffSec`, `pd.beepMode`

Store target:
- metadata: yes
- also useful as periodic telemetry so UI stays fresh

**11a. Storm Guard**
Common meaning:
- device-reported weather-protection mode that pre-charges supported EcoFlow systems ahead of outage risk

Examples:
- `Delta Pro 3` / `Delta 3 Plus` / `Delta 3 Max Plus` / `Delta Pro Ultra`: `stormPatternEnable`, `stormPatternOpenFlag`, `stormPatternEndTime`
- other documented EcoFlow variants: `stormIsEnable`, `inStormMode`, `stormEndTimestamp`

Implementation rule:
- treat Storm Guard as active only when the device reports both enable + open/in-storm flags
- do not infer Storm Guard from weak PV/MPPT values alone
- use the reported end timestamp when present for user-facing copy such as `Storm Guard active for ~2h more`

Store target:
- live telemetry/device details: yes
- metadata/capabilities: no special static capability flag required if the field family is already present in normalized groups

**12. Smart Generator / EV / Ecosystem Features**
Common meaning:
- Smart Generator thresholds, EV charging, extra battery ports, parallel box

Examples:
- `Delta Pro 3`: `cmsOilSelfStart`, `cmsOilOnSoc`, `cmsOilOffSoc`, extra battery port fields
- `Delta Pro Ultra`: `...backend_addr.EVMaxChargerCur`, `outPrPwr`, battery pack info, kit info
- `Delta 2 Max`: `bms_emsStatus.minOpenOilEb`, `maxCloseOilEb`, XT150 power, parallel-related fields

Store target:
- metadata: yes
- telemetry: only active runtime values

**13. Firmware / Network / Identity**
Common meaning:
- versions, wifi, 4G, timezone, ICCID, attached SNs

Examples:
- `Delta Pro 3`: `bleStandbyTime`, attached `plugInInfo...Sn`
- `Delta Pro Ultra`: `wireless4GSta`, `wireless4gCon`, `wireless4gOn`, `wirlesss4gErrCode`, `simIccid`, `sysTimezone`, `sysTimezoneId`
- `Delta 2 Max`: `pd.wifiVer`, `pd.wifiAutoRcvy`, `bms_bmsStatus.hwVersion`, `bms_bmsStatus.sysVer`

Store target:
- metadata: yes
- capabilities: some versioned capability flags

Implementation guidance from this grouping:

**Publish into telemetry pipeline**
These should become normalized `payload.params.*` and flow through NATS, projection, rollups, and DB:
- aggregate power
- PV watts/volts/amps/state
- battery watts/volts/amps/SOC/temp
- output/load powers
- remaining time
- charge/discharge state
- key switch states

**Persist into `provider_devices.capabilities`**
Mostly static or semi-static:
- battery slot count / pack count
- PV port count and per-port limits
- AC/DC/USB/Type-C/Anderson/XT150 presence
- extra battery / parallel / EV / generator support
- max charge/discharge capabilities

**Persist into `provider_devices.metadata`**
Dynamic but not replay-critical:
- reserve/backup settings
- SOC window / charge-discharge limits
- AC always-on / solar priority / xboost / energy management
- standby timers / screen settings
- firmware/network/timezone info
- attached accessory SNs

Most useful prefix-to-domain mapping:

- `Delta Pro 3`
  - `cms*` -> overall battery/SOC/ETA/limits
  - `bms*` -> main battery metrics
  - `pow*` -> live power
  - `plugInInfo*` -> port topology/capabilities
  - `flowInfo*` -> port on/off state
  - standalone flags -> settings/policy

- `Delta Pro Ultra`
  - `hs_yj751_pd_appshow_addr.*` -> high-level runtime telemetry
  - `hs_yj751_pd_app_set_info_addr.*` -> settings/policy
  - `hs_yj751_pd_backend_addr.*` -> low-level electrical telemetry
  - `hs_yj751_pd_bp_addr.bpInfo` -> pack topology
  - `hs_yj751_bms_slave_addr_*` -> per-pack telemetry
  - `bms_kitInfo.*` -> ecosystem battery/kit data

- `Delta 2 Max`
  - `pd.*` -> product/system/runtime summary
  - `bms_bmsStatus.*` -> battery pack telemetry
  - `bms_emsStatus.*` -> battery policy / reserve / EMS state
  - `inv.*` -> AC/inverter telemetry and config
  - `mppt.*` -> solar/PV/DC input telemetry and state

This is enough to drive the normalizer cleanly.

Recommended next code move:
- build one shared quota normalizer that emits three outputs:
  - `params`
  - `capabilities`
  - `metadata`

That keeps the mapping explicit and avoids scattering device-specific field handling across the worker.
