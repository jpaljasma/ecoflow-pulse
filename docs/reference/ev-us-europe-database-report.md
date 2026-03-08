# Reference: EV Database Package (United States + Europe)

Generated: 2026-03-08T13:16:47Z

## What is included

This package contains two machine-readable region layers:

1. **`us_vehicles`** — an official-current U.S. BEV layer built from the EPA / FuelEconomy downloadable vehicle dataset for model years **2025–2026**.
2. **`europe_vehicles`** — a Europe-oriented technical BEV layer built from **Open EV Data** for passenger cars with `release_year >= 2023`, then cross-checked against public Europe market references (EAFO, BEV Database, EV Database).

## Why these sources

### United States
The EPA / FuelEconomy site publishes downloadable vehicle datasets covering model years **1984–2026**, and the download page was updated on **February 20, 2026**. That makes it the cleanest public machine-ingestible source for current U.S. BEV range and consumption data.  
Source: FuelEconomy download page — [fueleconomy.gov/feg/download.shtml](https://www.fueleconomy.gov/feg/download.shtml)

Edmunds is useful as a **real-world** complement, not as the canonical machine-ingestible source. Their January 2026 EV range test page explicitly compares **EPA estimate vs. Edmunds tested range** and shows **EPA estimate vs. Edmunds tested consumption**.  
Source: Edmunds — [edmunds.com/car-news/electric-car-range-and-consumption-epa-vs-edmunds.html](https://www.edmunds.com/car-news/electric-car-range-and-consumption-epa-vs-edmunds.html)

### Europe
For Europe, I found three strong public references:

- **EAFO (European Alternative Fuels Observatory)** — official European Commission-operated page stating it tracks the electric passenger models **currently available in the EU**.  
  Source: [alternative-fuels-observatory.ec.europa.eu/.../available-electric-vehicle-models](https://alternative-fuels-observatory.ec.europa.eu/markets-and-policy/market-and-consumer-insights/available-electric-vehicle-models)

- **BEV Database** — public Europe EV comparison site that describes itself as covering Europe EV efficiency / charging / WLTP-oriented comparison metrics and says its table is updated monthly.  
  Source: [bev-database.com/cars-list](https://bev-database.com/cars-list)

- **EV Database** — public EU EV database that shows model availability plus technical fields like battery, fast charge and range.  
  Source: [ev-database.org](https://ev-database.org/)

The problem is that the public Europe sites above are **great to browse**, but I did **not** surface a clean free bulk JSON / CSV export of all current Europe trims with explicit WLTP-per-trim fields. To still produce an ingestible file, I used **Open EV Data** as the technical Europe layer because it is downloadable and structured.  
Source: Open EV Data raw JSON — [raw.githubusercontent.com/KilowattApp/open-ev-data/master/data/ev-data.json](https://raw.githubusercontent.com/KilowattApp/open-ev-data/master/data/ev-data.json)

## Important caveats

- **U.S. rows are official and high-confidence** for current market coverage.
- **Europe rows are best-effort technical references**, not a perfect official “all current EU trims with explicit WLTP labels” extraction.
- For that reason, each record keeps a **`range_cycle`** field:
  - U.S. rows use **`EPA combined`**
  - Europe rows use **`source-listed (not explicitly labeled WLTP in Open EV Data)`**
- Battery, AC/DC charging and average consumption fields in the Europe layer are still extremely useful programmatically, but you should treat the Europe layer as a **technical reference dataset** rather than a regulatory WLTP register.

## Package counts

- U.S. records: **617**
- Europe records: **550**
- U.S. records enriched with Open EV Data battery / charging fields: **192**
- Europe records with a populated source range field: **247**

## JSON structure

Top-level keys:

- `generated_at`
- `package_name`
- `methodology_version`
- `scope`
- `counts`
- `source_catalog`
- `us_vehicles`
- `europe_vehicles`

### U.S. row fields
availability_basis, availability_confidence, charge_time_120v_h, charge_time_240v_h, consumption_kwh_per_100km_combined, consumption_kwh_per_100mi_city, consumption_kwh_per_100mi_combined, consumption_kwh_per_100mi_highway, drive_layout, make, market, model, motor_description, mpge_city, mpge_combined, mpge_highway, range_city_mi, range_cycle, range_highway_mi, range_km, range_mi, source_ids, technical_enrichment, transmission, vehicle_class, year

### Europe row fields
ac_max_power_kw, ac_phases, ac_ports, availability_basis, availability_confidence, average_consumption_kwh_per_100km, battery_type, charging_voltage_v, dc_max_power_kw, dc_ports, make, market, model, nominal_battery_kwh, range_cycle, range_km, release_year, source_ids, usable_battery_kwh, variant, vehicle_type

## Sample: longest-range U.S. records in this package

|   year | make      | model                                            |   range_mi |   consumption_kwh_per_100mi_combined | drive_layout    |
|-------:|:----------|:-------------------------------------------------|-----------:|-------------------------------------:|:----------------|
|   2025 | Lucid     | Air G Touring XR AWD with 19 inch wheels         |        512 |                              26.4283 | All-Wheel Drive |
|   2026 | Lucid     | Air G Touring XR AWD with19 inch wheels          |        512 |                              26.4283 | All-Wheel Drive |
|   2026 | Chevrolet | Silverado EV 24-mod battery, 19kW 6-mode charger |        493 |                              49.4829 | 4-Wheel Drive   |
|   2025 | Chevrolet | Silverado EV 8WT                                 |        492 |                              49.881  | 4-Wheel Drive   |
|   2025 | Lucid     | Air G Touring XR AWD with 20 inch wheels         |        480 |                              28.1608 | All-Wheel Drive |
|   2026 | Lucid     | Air G Touring XR AWD with 20 inch wheels         |        480 |                              28.1608 | All-Wheel Drive |
|   2025 | Lucid     | Gravity GT w/20F21R wheels (2R)                  |        450 |                              31.0845 | All-Wheel Drive |
|   2026 | Lucid     | Gravity GT w/20F21R wheels (2R)                  |        450 |                              31.0948 | All-Wheel Drive |
|   2025 | Lucid     | Air G Touring XR AWD with 21 inch wheels         |        446 |                              30.1254 | All-Wheel Drive |
|   2026 | Lucid     | Air G Touring XR AWD with 21 inch wheels         |        446 |                              30.1254 | All-Wheel Drive |
|   2025 | Lucid     | Gravity GT w/20F21R wheels (3R)                  |        437 |                              32.4302 | All-Wheel Drive |
|   2026 | Lucid     | Gravity GT w/20F21R wheels (3R)                  |        437 |                              32.4302 | All-Wheel Drive |

## Sample: lowest-consumption Europe records in this package

|   release_year | make      | model                   | variant        |   average_consumption_kwh_per_100km |   usable_battery_kwh | range_km   |
|---------------:|:----------|:------------------------|:---------------|------------------------------------:|---------------------:|:-----------|
|           2024 | Leapmotor | T03                     |                |                                13.6 |                 37.3 | 230.0      |
|           2023 | Tesla     | Model 3                 |                |                                13.7 |                 57.5 |            |
|           2024 | Tesla     | Model 3                 | Long Range RWD |                                13.8 |                 75   |            |
|           2023 | Tesla     | 3 long range dual motor |                |                                14.3 |                 75   |            |
|           2023 | Mini      | Cooper SE Convertible   |                |                                14.5 |                 32.6 | 165.0      |
|           2024 | Mini      | Cooper                  | E              |                                14.6 |                 36.6 |            |
|           2023 | Mini      | Cooper E                |                |                                14.6 |                 36.6 |            |
|           2025 | Kia       | EV4 Sedan Long Range    |                |                                14.8 |                 72.8 | 495.0      |
|           2024 | Citroën   | e-C4 X 54 kWh           |                |                                14.9 |                 50.8 | 340.0      |
|           2023 | Hyundai   | 6 standard range 2wd    |                |                                14.9 |                 50   |            |
|           2024 | Lucid     | Air Pure RWD            |                |                                14.9 |                 92   | 565.0      |
|           2024 | Mini      | Cooper                  | SE             |                                14.9 |                 49.2 |            |

## Suggested usage

- Use **`us_vehicles`** when you need an official U.S. market layer for range / consumption analysis.
- Use **`europe_vehicles`** when you need battery / charging / consumption enrichment for Europe-oriented BEV analysis.
- If you need **strict per-trim WLTP** values for Europe, the cleanest next step would be a second-pass build from OEM pages or a licensed Europe EV data feed.

## Source catalog

### 1) FuelEconomy / EPA
- URL: [fueleconomy.gov/feg/download.shtml](https://www.fueleconomy.gov/feg/download.shtml)
- Role: official U.S. machine-ingestible source

### 2) Edmunds Tested: Electric Car Range and Consumption
- URL: [edmunds.com/car-news/electric-car-range-and-consumption-epa-vs-edmunds.html](https://www.edmunds.com/car-news/electric-car-range-and-consumption-epa-vs-edmunds.html)
- Role: U.S. real-world validation / comparison

### 3) European Alternative Fuels Observatory (EAFO)
- URL: [alternative-fuels-observatory.ec.europa.eu/.../available-electric-vehicle-models](https://alternative-fuels-observatory.ec.europa.eu/markets-and-policy/market-and-consumer-insights/available-electric-vehicle-models)
- Role: official EU current-availability reference

### 4) BEV Database
- URL: [bev-database.com/cars-list](https://bev-database.com/cars-list)
- Role: Europe public comparison database

### 5) EV Database
- URL: [ev-database.org](https://ev-database.org/)
- Role: Europe public technical comparison database

### 6) Open EV Data
- URL: [raw.githubusercontent.com/KilowattApp/open-ev-data/master/data/ev-data.json](https://raw.githubusercontent.com/KilowattApp/open-ev-data/master/data/ev-data.json)
- Role: downloadable structured technical EV dataset used for the Europe layer and U.S. enrichment
