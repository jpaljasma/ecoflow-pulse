# Reference: Solar Avoided Emissions

This document defines a practical, research-backed way to estimate avoided
emissions from home solar generation.

It is designed for use in EcoFlow Pulse when you know or can estimate:

- `annualSolarKWh`, or any other real measured solar period in `kWh`
- the user's eGRID subregion (preferred), or
- a fallback U.S. average factor

Current UI note:

- the shipped `Energy Impact` widget on `/devices` and `/device/{id}` uses
  real `today so far` solar generation already measured in the app
- the same formulas below apply to any period once month/year/lifetime totals
  are exposed in the dashboard
- the widget also exposes a separate tree-equivalent row based on PV lifecycle
  CO2 benchmark; see [`tree-equivalent.md`](tree-equivalent.md)

## Recommended method

For avoided emissions from rooftop solar, use EPA eGRID non-baseload output
emission rates.

Why this is the right default:

- EPA says eGRID subregion-level factors best represent the regional
  electricity actually consumed
- EPA says non-baseload output emission rates can be used to estimate
  emissions avoided by projects that displace marginal fossil generation,
  including renewable energy
- this makes non-baseload eGRID factors the best simple default for a product
  feature like "your solar avoided X emissions"

### Important interpretation note

This method estimates grid emissions avoided at the point of displaced
generation. It is not the same thing as a formal corporate Scope 2 inventory
method.

## Default factors

### 1) Upstate New York (NYUP) - recommended default for Naples, NY

From EPA eGRID2023 Subregion Output Emission Rates:

- `co2eLbPerMWh = 911.8`
- `noxLbPerMWh = 0.4`
- `so2LbPerMWh = 0.103`

Converted to per-kWh app constants:

- `co2eKgPerKWh = 0.413585523`
- `co2eMetricTonsPerKWh = 0.000413585523`
- `noxGramsPerKWh = 0.181436948`
- `so2GramsPerKWh = 0.046720014`

### 2) U.S. average fallback

From EPA eGRID2023 Subregion Output Emission Rates:

- `co2eLbPerMWh = 1379.2`
- `noxLbPerMWh = 0.9`
- `so2LbPerMWh = 0.684`

Converted to per-kWh app constants:

- `co2eKgPerKWh = 0.625594597`
- `co2eMetricTonsPerKWh = 0.000625594597`
- `noxGramsPerKWh = 0.408233133`
- `so2GramsPerKWh = 0.310257181`

## Core formulas

Let:

- `X = solarKWhForPeriod`
- `Fco2 = co2eMetricTonsPerKWh`
- `Fnox = noxGramsPerKWh`
- `Fso2 = so2GramsPerKWh`

Then:

```text
avoidedCO2eMetricTons = X * Fco2
avoidedNOxGrams       = X * Fnox
avoidedSO2Grams       = X * Fso2
```

### Human-friendly display helpers

```text
avoidedCO2eKg = avoidedCO2eMetricTons * 1000
avoidedNOxKg  = avoidedNOxGrams / 1000
avoidedSO2Kg  = avoidedSO2Grams / 1000
```

## Ready-to-use constants

```json
{
  "avoidedEmissionsFactors": {
    "NYUP": {
      "label": "NPCC Upstate NY",
      "source": "EPA eGRID2023 non-baseload output emission rates",
      "co2eMetricTonsPerKWh": 0.000413585523,
      "co2eKgPerKWh": 0.413585523,
      "noxGramsPerKWh": 0.181436948,
      "so2GramsPerKWh": 0.046720014
    },
    "US_AVG": {
      "label": "U.S. average",
      "source": "EPA eGRID2023 non-baseload output emission rates",
      "co2eMetricTonsPerKWh": 0.000625594597,
      "co2eKgPerKWh": 0.625594597,
      "noxGramsPerKWh": 0.408233133,
      "so2GramsPerKWh": 0.310257181
    }
  }
}
```

## Example calculations

### Example A - 10,000 kWh/year in Upstate New York (NYUP)

```text
avoidedCO2eMetricTons = 10000 * 0.000413585523 = 4.13585523 tCO2e/yr
avoidedNOxGrams       = 10000 * 0.181436948    = 1814.36948 g/yr
avoidedSO2Grams       = 10000 * 0.046720014    = 467.200141 g/yr
```

Display version:

- 4.14 metric tons CO2e avoided / year
- 1.81 kg NOx avoided / year
- 0.47 kg SO2 avoided / year

### Example B - 10,000 kWh/year using U.S. average

```text
avoidedCO2eMetricTons = 10000 * 0.000625594597 = 6.25594597 tCO2e/yr
avoidedNOxGrams       = 10000 * 0.408233133    = 4082.33133 g/yr
avoidedSO2Grams       = 10000 * 0.310257181    = 3102.57181 g/yr
```

Display version:

- 6.26 metric tons CO2e avoided / year
- 4.08 kg NOx avoided / year
- 3.10 kg SO2 avoided / year

## Optional climate note: net lifecycle CO2e benefit

If you want a more conservative climate number, you can subtract the lifecycle
emissions of crystalline-silicon PV.

A well-known harmonization study found a median of `45 g CO2e/kWh` for
crystalline-silicon PV after harmonization. NREL also reported newer
utility-scale PV results in the `10-36 g CO2e/kWh` range, which suggests `45 g`
is conservative for modern systems.

### Conservative lifecycle adjustment

```text
pvLifecycleKgPerKWh = 0.045
netAvoidedCO2eKg = solarKWhForPeriod * (gridCO2eKgPerKWh - pvLifecycleKgPerKWh)
```

#### Net lifecycle example - NYUP

```text
netAvoidedCO2eKg = X * (0.413585523 - 0.045)
                 = X * 0.368585523
```

For `X = 10000 kWh/year`:

```text
netAvoidedCO2e = 3.68585523 metric tons/year
```

### Recommendation for Pulse UX

Use:

- primary metric: EPA grid-avoided emissions
- optional secondary note: "Net of PV lifecycle manufacturing emissions, the
  climate benefit is still approximately ..."

Do not subtract PV lifecycle values from NOx or SO2 in the UI unless you are
building a dedicated lifecycle model for those pollutants.

## Suggested product copy

### Short UI text

> Your solar generation displaced fossil power on the grid. Based on EPA
> regional marginal-grid factors, your system avoided approximately
> **{co2eTons} tons CO2e**, **{noxKg} kg NOx**, and **{so2Kg} kg SO2** over the
> selected period.

### Tooltip / explainer text

> Avoided emissions are estimated using EPA eGRID non-baseload emission rates,
> which approximate the power plants that ramp up and down as electricity
> demand changes. This is a practical estimate of pollution avoided by your
> solar generation.

## Implementation notes

1. Prefer eGRID subregion factors over national averages.
2. If user ZIP-to-subregion mapping is unavailable, use `US_AVG` and label it
   clearly.
3. Round for UI display:
   - CO2e: 2 decimals in metric tons for large ranges
   - NOx/SO2: 2 decimals in kg or grams for small values
4. Recompute on any date-range change:
   - today
   - month
   - year
   - lifetime
5. Keep the factor year versioned in code, e.g. `egrid2023_rev2`.
6. Current shipped dashboard integration uses today-so-far measured solar only.

## Sources

1. EPA eGRID FAQ - explains that subregion rates are preferred for
   electricity-use geography, and that non-baseload output emission rates can
   be used to estimate emissions avoided by renewable energy and efficiency
   projects.
   [EPA eGRID FAQ](https://www.epa.gov/egrid/frequent-questions-about-egrid)
2. EPA eGRID2023 Summary Tables (rev2) - source for the NYUP and U.S.
   non-baseload CO2e, NOx, and SO2 factors used above.
   [EPA eGRID2023 Summary Tables rev2](https://www.epa.gov/system/files/documents/2025-06/summary_tables_rev2.pdf)
3. Hsu et al. - Life Cycle Greenhouse Gas Emissions of Crystalline Silicon
   Photovoltaic Electricity Generation.
   [Hsu et al. harmonization paper](https://ultralowcarbonsolar.org/assets/Life%20Cycle%20Greenhouse%20Gas%20Emissions%20of%20Crystalline%20Silicon%20PhotovoltaicElectricity%20GenerationSystematic%20Review%20and%20Harmonization.pdf)
4. NREL 2024 utility-scale PV LCA update.
   [NREL 2024 utility-scale PV LCA update](https://docs.nrel.gov/docs/fy24osti/87372.pdf)

## Source excerpts used for this implementation

### EPA eGRID2023 (NYUP non-baseload)

- CO2e = `911.8 lb/MWh`
- Annual NOx = `0.4 lb/MWh`
- SO2 = `0.103 lb/MWh`

### EPA eGRID2023 (U.S. non-baseload)

- CO2e = `1379.2 lb/MWh`
- Annual NOx = `0.9 lb/MWh`
- SO2 = `0.684 lb/MWh`

### PV lifecycle climate factor

- Crystalline silicon PV median = `45 g CO2e/kWh`

## Copy-paste formula block

```text
Given solarKWhForPeriod = X

NYUP:
  avoidedCO2eMetricTons = X * 0.000413585523
  avoidedNOxGrams       = X * 0.181436948
  avoidedSO2Grams       = X * 0.046720014

US average:
  avoidedCO2eMetricTons = X * 0.000625594597
  avoidedNOxGrams       = X * 0.408233133
  avoidedSO2Grams       = X * 0.310257181
```
