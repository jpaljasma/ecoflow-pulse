# Reference: Tree Equivalent

This document converts the PV lifecycle carbon intensity benchmark of
`0.045 kg CO2e per kWh` into an easy-to-understand tree-equivalent metric for
use in EcoFlow Pulse.

Important:

- this is a `kWh/year` equivalence, not `kW`
- trees remove a quantity of CO2 over time, so the useful comparison is how
  many kilowatt-hours per year of solar generation have the same lifecycle CO2
  as one tree removes in one year

Current UI note:

- the shipped `Energy Impact` widget uses real `today so far` solar generation
- the tree row is a lifecycle-CO2 comparison only
- it does not represent avoided `NOx` or `SO2`

## Core conversion

Using the crystalline-silicon PV lifecycle benchmark:

- PV lifecycle emissions: `0.045 kg CO2e / kWh`

Formula:

```text
Tree-equivalent solar generation (kWh/year) = tree CO2 removed per year (kg) / 0.045
```

Inverse form:

```text
Tree-years equivalent = (solar generation in kWh × 0.045) / tree CO2 removed per year (kg)
```

## Conservative generic mature-tree benchmark

A widely cited USDA / Arbor Day benchmark says a mature tree absorbs more than
`48 lb` of CO2 per year, which is about `21.8 kg CO2/year`.

That gives:

```text
21.8 / 0.045 = 484 kWh/year
```

### Recommended conservative app copy

- `1 mature tree-year ≈ 484 kWh of solar lifecycle CO2 offset`

This is the safest general-purpose number to show if you want a simple,
conservative public-facing metric.

## Species-shaped examples

EPA’s urban/suburban tree sequestration method gives annual sequestration
values by growth class at about age `40` after planting. These values are for
open-grown urban/suburban trees, so they can be significantly higher than the
generic mature-tree benchmark above.

### White spruce

EPA classifies white spruce as `conifer / moderate`.

- annual sequestration: `41.5 lb carbon / year`
- converted to CO2: about `69.1 kg CO2 / year`
- solar-equivalent:

```text
69.1 / 0.045 = 1,535 kWh/year
```

Result:

- `1 mature white spruce-year ≈ 1,535 kWh/year`

### Paper birch

EPA classifies paper birch as `hardwood / moderate`.

- annual sequestration: `51.7 lb carbon / year`
- converted to CO2: about `86.1 kg CO2 / year`
- solar-equivalent:

```text
86.1 / 0.045 = 1,913 kWh/year
```

Result:

- `1 mature paper birch-year ≈ 1,913 kWh/year`

### Oak

Oak varies a lot by species and growth class.

#### Slow oak

Examples include species EPA groups into `hardwood / slow`.

- annual sequestration: `23.4 lb carbon / year`
- converted to CO2: about `39.0 kg CO2 / year`
- solar-equivalent:

```text
39.0 / 0.045 = 866 kWh/year
```

Result:

- `1 slow-growth mature oak-year ≈ 866 kWh/year`

#### Moderate oak

Examples include species EPA groups into `hardwood / moderate`.

- annual sequestration: `51.7 lb carbon / year`
- converted to CO2: about `86.1 kg CO2 / year`
- solar-equivalent:

```text
86.1 / 0.045 = 1,913 kWh/year
```

Result:

- `1 moderate-growth mature oak-year ≈ 1,913 kWh/year`

#### Fast oak

Examples include species EPA groups into `hardwood / fast`.

- annual sequestration: `93.2 lb carbon / year`
- converted to CO2: about `155.2 kg CO2 / year`
- solar-equivalent:

```text
155.2 / 0.045 = 3,448 kWh/year
```

Result:

- `1 fast-growth mature oak-year ≈ 3,448 kWh/year`

## Practical recommendations for EcoFlow Pulse

### Best default for public UI

Use the conservative generic tree number:

- `1 mature tree-year ≈ 484 kWh`

Reason: it is easier to explain, less likely to overstate, and avoids
species/location assumptions.

### Good advanced or educational UI copy

You can optionally show a range:

- conservative mature tree: about `484 kWh/year`
- spruce: about `1,535 kWh/year`
- birch: about `1,913 kWh/year`
- oak: about `866-3,448 kWh/year` depending on species

### Suggested tooltip text

```text
Tree equivalent compares the lifecycle CO2 of solar electricity to the amount of CO2 a mature tree removes in one year. Default estimate uses a conservative generic mature-tree benchmark.
```

### Suggested disclaimer text

```text
Tree-equivalent values are approximate. Actual annual CO2 removal depends heavily on tree species, age, size, climate, and growing conditions. This metric reflects CO2 only and does not represent removal of NOx, SO2, particulates, or other air pollutants.
```

## Worked examples

### Example 1: 1,000 kWh of solar generation

Lifecycle CO2:

```text
1,000 × 0.045 = 45 kg CO2e
```

Equivalent generic mature tree-years:

```text
45 / 21.8 = 2.06 tree-years
```

Equivalent mature white spruce-years:

```text
45 / 69.1 = 0.65 spruce-years
```

### Example 2: 10,000 kWh of solar generation

Lifecycle CO2:

```text
10,000 × 0.045 = 450 kg CO2e
```

Equivalent generic mature tree-years:

```text
450 / 21.8 = 20.6 tree-years
```

Equivalent paper birch-years:

```text
450 / 86.1 = 5.2 birch-years
```

## Notes and limitations

1. This is a lifecycle-CO2 comparison only. It does not mean trees remove
   solar’s avoided `NOx`, `SO2`, particulates, or mercury in the same way.
2. Species numbers vary a lot. Growth rate, size, local climate, planting
   density, and whether the tree is open-grown or forest-grown all matter.
3. The generic mature-tree benchmark is more conservative. It is usually the
   better default if the goal is stable, low-risk public communication.
4. PV lifecycle intensity varies by technology and manufacturing pathway. The
   `45 g CO2e/kWh` value is a conservative benchmark for crystalline-silicon
   PV.

## Sources

1. NREL-led harmonized review for crystalline-silicon PV lifecycle emissions.
   Benchmark used here: about `45 g CO2e/kWh`.
   [Hsu et al. harmonized review](https://ultralowcarbonsolar.org/assets/Life%20Cycle%20Greenhouse%20Gas%20Emissions%20of%20Crystalline%20Silicon%20PhotovoltaicElectricity%20GenerationSystematic%20Review%20and%20Harmonization.pdf)
2. USDA / Arbor Day benchmark for mature tree annual CO2 uptake.
   [USDA tree benchmark](https://www.usda.gov/about-usda/news/blog/power-one-tree-very-air-we-breathe)
3. EPA method for carbon sequestration by urban and suburban trees.
   [EPA sequestration method](https://www3.epa.gov/climatechange/Downloads/method-calculating-carbon-sequestration-trees-urban-and-suburban-settings.pdf)

## Pulse-ready summary block

```text
Using a conservative PV lifecycle benchmark of 0.045 kg CO2e per kWh, one mature tree-year is roughly equivalent to 484 kWh of solar generation. Species-specific examples vary widely: white spruce ≈ 1,535 kWh/year, paper birch ≈ 1,913 kWh/year, and oak ≈ 866–3,448 kWh/year depending on species and growth class.
```
