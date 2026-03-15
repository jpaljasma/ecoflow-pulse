import { useMemo, useState } from 'react';
import { Platform, Pressable, View } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { Canvas, Path, Skia } from '@shopify/react-native-skia';
import {
  SOLAR_HISTORY_BUCKET_MINUTES,
  SOLAR_HISTORY_END_HOUR,
  SOLAR_HISTORY_POINTS,
  SOLAR_HISTORY_START_HOUR,
  SOLAR_HISTORY_TICK_HOURS
} from '@/features/history/solar';
import { formatW, formatWhAndKWh } from '@/features/telemetry/format';
import { useThemeSemantics } from '@/shared/theme/semantic';
import { formatSolarLegendDelta } from '@/shared/ui/solarLegend';

const CHART_HEIGHT = 170;
const WEB_CHART_HEIGHT = 210;
const PAD_X = 8;
const PAD_Y = 14;
const EPSILON = 1e-6;
const Y_AXIS_WIDTH = 44;
const X_AXIS_LABEL_WIDTH = 40;
const TOOLTIP_WIDTH = 208;
const TOOLTIP_TOP = 8;
const SELECTION_DOT_SIZE = 8;
const X_AXIS_TICKS = SOLAR_HISTORY_TICK_HOURS.map((hour) => ({
  label: `${String(hour).padStart(2, '0')}:00`,
  fraction: (hour - SOLAR_HISTORY_START_HOUR) / (SOLAR_HISTORY_END_HOUR - SOLAR_HISTORY_START_HOUR)
}));
type Point = { x: number; y: number };

function looksCumulative(values: number[]): boolean {
  if (values.length < 8) return false;
  let nonDecreasing = 0;
  for (let i = 1; i < values.length; i += 1) {
    if ((values[i] ?? 0) + EPSILON >= (values[i - 1] ?? 0)) nonDecreasing += 1;
  }
  const monotonicRatio = nonDecreasing / Math.max(1, values.length - 1);
  const first = values[0] ?? 0;
  const last = values[values.length - 1] ?? 0;
  return monotonicRatio >= 0.92 && last > first + 1;
}

function toBucketSeries(values: number[]): number[] {
  if (!looksCumulative(values)) return values;
  const buckets: number[] = [];
  let prev = Math.max(0, values[0] ?? 0);
  for (let i = 0; i < values.length; i += 1) {
    const curr = Math.max(0, values[i] ?? 0);
    const delta = i === 0 ? curr : Math.max(0, curr - prev);
    buckets.push(delta);
    prev = curr;
  }
  return buckets;
}

function toWattsFromWhPerBucket(valueWh: number): number {
  // 10-minute bucket energy (Wh) => average bucket power (W)
  return Math.max(0, valueWh) * 6;
}

function roundAxisMaxWatts(value: number): number {
  if (!Number.isFinite(value) || value <= 0) return 100;
  if (value <= 100) return Math.ceil(value / 10) * 10;
  if (value <= 500) return Math.ceil(value / 25) * 25;
  if (value <= 2000) return Math.ceil(value / 50) * 50;
  return Math.ceil(value / 100) * 100;
}

function formatAxisW(value: number): string {
  if (value >= 1000) return `${(value / 1000).toFixed(1)}kW`;
  return `${Math.round(value)}W`;
}

function pointAt(points: Point[], idx: number): Point {
  const clamped = Math.max(0, Math.min(points.length - 1, idx));
  return points[clamped] ?? { x: 0, y: 0 };
}

function buildPoints(values: number[], width: number, height: number, max: number): Point[] {
  if (values.length < 2 || width <= 0 || height <= 0) return [];
  const plotW = Math.max(1, width - PAD_X * 2);
  const plotH = Math.max(1, height - PAD_Y * 2);
  const range = Math.max(1e-9, max);
  return values.map((value, idx) => ({
    x: PAD_X + (idx / (values.length - 1)) * plotW,
    y: height - PAD_Y - (Math.max(0, value) / range) * plotH
  }));
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}

function bucketIndexFromLocationX(locationX: number, width: number, points: number): number {
  const plotW = Math.max(1, width - PAD_X * 2);
  const fraction = clamp((locationX - PAD_X) / plotW, 0, 1);
  return Math.round(fraction * Math.max(0, points - 1));
}

function bucketCenterX(index: number, width: number, points: number): number {
  const plotW = Math.max(1, width - PAD_X * 2);
  return PAD_X + (index / Math.max(1, points - 1)) * plotW;
}

function formatBucketClock(totalMinutes: number): string {
  const hours = Math.floor(totalMinutes / 60);
  const minutes = totalMinutes % 60;
  return `${String(hours).padStart(2, '0')}:${String(minutes).padStart(2, '0')}`;
}

function formatBucketRangeLabel(index: number): string {
  const startMinutes = SOLAR_HISTORY_START_HOUR * 60 + index * SOLAR_HISTORY_BUCKET_MINUTES;
  const endMinutes = startMinutes + SOLAR_HISTORY_BUCKET_MINUTES;
  return `${formatBucketClock(startMinutes)} - ${formatBucketClock(endMinutes)}`;
}

function tooltipLeftForX(x: number, width: number): number {
  return clamp(x - TOOLTIP_WIDTH / 2, 4, Math.max(4, width - TOOLTIP_WIDTH - 4));
}

function buildSkiaSmoothPath(points: Point[]) {
  if (points.length < 2) return null;
  const path = Skia.Path.Make();
  const first = pointAt(points, 0);
  path.moveTo(first.x, first.y);

  for (let i = 0; i < points.length - 1; i += 1) {
    const p0 = pointAt(points, i - 1);
    const p1 = pointAt(points, i);
    const p2 = pointAt(points, i + 1);
    const p3 = pointAt(points, i + 2);
    const cp1x = p1.x + (p2.x - p0.x) / 6;
    const cp1y = p1.y + (p2.y - p0.y) / 6;
    const cp2x = p2.x - (p3.x - p1.x) / 6;
    const cp2y = p2.y - (p3.y - p1.y) / 6;
    path.cubicTo(cp1x, cp1y, cp2x, cp2y, p2.x, p2.y);
  }

  return path;
}

function buildSvgSmoothPath(points: Point[]): string {
  if (points.length < 2) return '';
  const first = pointAt(points, 0);
  let d = `M ${first.x.toFixed(2)} ${first.y.toFixed(2)}`;
  for (let i = 0; i < points.length - 1; i += 1) {
    const p0 = pointAt(points, i - 1);
    const p1 = pointAt(points, i);
    const p2 = pointAt(points, i + 1);
    const p3 = pointAt(points, i + 2);
    const cp1x = p1.x + (p2.x - p0.x) / 6;
    const cp1y = p1.y + (p2.y - p0.y) / 6;
    const cp2x = p2.x - (p3.x - p1.x) / 6;
    const cp2y = p2.y - (p3.y - p1.y) / 6;
    d += ` C ${cp1x.toFixed(2)} ${cp1y.toFixed(2)} ${cp2x.toFixed(2)} ${cp2y.toFixed(2)} ${p2.x.toFixed(2)} ${p2.y.toFixed(2)}`;
  }
  return d;
}

function buildDashedSvgPath(points: Point[], dashLength = 4, gapLength = 6): string {
  if (points.length < 2) return '';

  let d = '';
  let drawing = true;
  let remaining = dashLength;

  for (let index = 1; index < points.length; index += 1) {
    const start = pointAt(points, index - 1);
    const end = pointAt(points, index);
    const dx = end.x - start.x;
    const dy = end.y - start.y;
    const segmentLength = Math.hypot(dx, dy);
    if (segmentLength <= EPSILON) {
      continue;
    }

    let offset = 0;
    while (offset < segmentLength - EPSILON) {
      const chunk = Math.min(remaining, segmentLength - offset);
      const fromRatio = offset / segmentLength;
      const toRatio = (offset + chunk) / segmentLength;
      const fromX = start.x + dx * fromRatio;
      const fromY = start.y + dy * fromRatio;
      const toX = start.x + dx * toRatio;
      const toY = start.y + dy * toRatio;
      if (drawing) {
        d += ` M ${fromX.toFixed(2)} ${fromY.toFixed(2)} L ${toX.toFixed(2)} ${toY.toFixed(2)}`;
      }
      offset += chunk;
      remaining -= chunk;
      if (remaining <= EPSILON) {
        drawing = !drawing;
        remaining = drawing ? dashLength : gapLength;
      }
    }
  }

  return d.trim();
}

function buildDashedSkiaPath(points: Point[], dashLength = 4, gapLength = 6) {
  if (points.length < 2) return null;

  const path = Skia.Path.Make();
  let drawing = true;
  let remaining = dashLength;

  for (let index = 1; index < points.length; index += 1) {
    const start = pointAt(points, index - 1);
    const end = pointAt(points, index);
    const dx = end.x - start.x;
    const dy = end.y - start.y;
    const segmentLength = Math.hypot(dx, dy);
    if (segmentLength <= EPSILON) {
      continue;
    }

    let offset = 0;
    while (offset < segmentLength - EPSILON) {
      const chunk = Math.min(remaining, segmentLength - offset);
      const fromRatio = offset / segmentLength;
      const toRatio = (offset + chunk) / segmentLength;
      const fromX = start.x + dx * fromRatio;
      const fromY = start.y + dy * fromRatio;
      const toX = start.x + dx * toRatio;
      const toY = start.y + dy * toRatio;
      if (drawing) {
        path.moveTo(fromX, fromY);
        path.lineTo(toX, toY);
      }
      offset += chunk;
      remaining -= chunk;
      if (remaining <= EPSILON) {
        drawing = !drawing;
        remaining = drawing ? dashLength : gapLength;
      }
    }
  }

  return path;
}

function LegendLine({ color, dotted = false }: { color: string; dotted?: boolean }) {
  return (
    <View
      style={{
        width: 18,
        height: 0,
        borderTopWidth: dotted ? 1.5 : 2.5,
        borderColor: color,
        borderStyle: dotted ? 'dashed' : 'solid',
        marginTop: 8
      }}
    />
  );
}

function SelectionOverlay({
  width,
  height,
  selectedX,
  todayPoint,
  yesterdayPoint,
  bucketLabel,
  todayBucketWh,
  todayBucketW,
  yesterdayBucketWh,
  yesterdayBucketW,
  colors
}: {
  width: number;
  height: number;
  selectedX: number;
  todayPoint?: Point;
  yesterdayPoint?: Point;
  bucketLabel: string;
  todayBucketWh: number;
  todayBucketW: number;
  yesterdayBucketWh: number;
  yesterdayBucketW: number;
  colors: {
    crosshair: string;
    yesterdaySeries: string;
    selectionRingSoft: string;
    selectionRingStrong: string;
    solarSeries: string;
    tooltipBackground: string;
    tooltipBorder: string;
    tooltipTitle: string;
    tooltipToday: string;
    tooltipYesterday: string;
    tooltipUp: string;
    tooltipDown: string;
  };
}) {
  const tooltipLeft = tooltipLeftForX(selectedX, width);
  const todayBeatsYesterday = todayBucketWh >= yesterdayBucketWh;
  const comparisonGlyph = todayBeatsYesterday ? '^' : 'v';
  const comparisonColor = todayBeatsYesterday ? colors.tooltipUp : colors.tooltipDown;

  return (
    <View pointerEvents="none" style={{ position: 'absolute', top: 0, right: 0, bottom: 0, left: 0 }}>
      <View
        style={{
          position: 'absolute',
          left: selectedX - 0.5,
          top: PAD_Y,
          width: 1,
          height: Math.max(0, height - PAD_Y * 2),
          backgroundColor: colors.crosshair
        }}
      />
      {yesterdayPoint ? (
        <View
          style={{
            position: 'absolute',
            left: yesterdayPoint.x - SELECTION_DOT_SIZE / 2,
            top: yesterdayPoint.y - SELECTION_DOT_SIZE / 2,
            width: SELECTION_DOT_SIZE,
            height: SELECTION_DOT_SIZE,
            borderRadius: SELECTION_DOT_SIZE / 2,
            backgroundColor: colors.yesterdaySeries,
            borderWidth: 1,
            borderColor: colors.selectionRingSoft
          }}
        />
      ) : null}
      {todayPoint ? (
        <View
          style={{
            position: 'absolute',
            left: todayPoint.x - SELECTION_DOT_SIZE / 2,
            top: todayPoint.y - SELECTION_DOT_SIZE / 2,
            width: SELECTION_DOT_SIZE,
            height: SELECTION_DOT_SIZE,
            borderRadius: SELECTION_DOT_SIZE / 2,
            backgroundColor: colors.solarSeries,
            borderWidth: 1,
            borderColor: colors.selectionRingStrong
          }}
        />
      ) : null}
      <View
        style={{
          position: 'absolute',
          top: TOOLTIP_TOP,
          left: tooltipLeft,
          width: TOOLTIP_WIDTH,
          paddingVertical: 8,
          paddingHorizontal: 10,
          borderRadius: 10,
          backgroundColor: colors.tooltipBackground,
          borderWidth: 1,
          borderColor: colors.tooltipBorder
        }}
      >
        <Text fontSize="$1" marginBottom="$1" style={{ color: colors.tooltipTitle }}>
          {bucketLabel}
        </Text>
        <XStack alignItems="center" gap="$1">
          <Text fontSize="$1" style={{ color: comparisonColor }}>
            {comparisonGlyph}
          </Text>
          <Text fontSize="$1" style={{ color: colors.tooltipToday }}>
            Today: {formatWhAndKWh(todayBucketWh)} · {formatW(todayBucketW)}
          </Text>
        </XStack>
        <Text fontSize="$1" style={{ color: colors.tooltipYesterday }}>
          Yesterday: {formatWhAndKWh(yesterdayBucketWh)} · {formatW(yesterdayBucketW)}
        </Text>
      </View>
    </View>
  );
}

export function SolarGeneratedChart({
  valuesWh,
  yesterdayValuesWh,
  todayWh,
  yesterdayWh,
  deltaPct,
  points = SOLAR_HISTORY_POINTS
}: {
  valuesWh: number[] | undefined;
  yesterdayValuesWh?: number[] | undefined;
  todayWh?: number | null;
  yesterdayWh?: number | null;
  deltaPct?: number | null;
  points?: number;
}) {
  const [width, setWidth] = useState(0);
  const [selectedIndex, setSelectedIndex] = useState<number | null>(null);
  const semantics = useThemeSemantics();
  const seriesBucketWh = useMemo(() => {
    const trimmed = (valuesWh ?? []).slice(-points).map((v) => Math.max(0, v));
    const padded =
      trimmed.length >= points
        ? trimmed
        : [...Array.from({ length: points - trimmed.length }, () => 0), ...trimmed];
    return toBucketSeries(padded);
  }, [points, valuesWh]);
  const yesterdaySeriesBucketWh = useMemo(() => {
    const trimmed = (yesterdayValuesWh ?? []).slice(-points).map((v) => Math.max(0, v));
    const padded =
      trimmed.length >= points
        ? trimmed
        : [...Array.from({ length: points - trimmed.length }, () => 0), ...trimmed];
    return toBucketSeries(padded);
  }, [points, yesterdayValuesWh]);
  const seriesW = useMemo(() => seriesBucketWh.map(toWattsFromWhPerBucket), [seriesBucketWh]);
  const yesterdaySeriesW = useMemo(
    () => yesterdaySeriesBucketWh.map(toWattsFromWhPerBucket),
    [yesterdaySeriesBucketWh]
  );
  const maxVal = useMemo(
    () =>
      roundAxisMaxWatts(
        [...seriesW, ...yesterdaySeriesW].reduce((acc, value) => Math.max(acc, value), 0)
      ),
    [seriesW, yesterdaySeriesW]
  );
  const yAxisLabels = useMemo(() => [maxVal, maxVal / 2, 0], [maxVal]);
  const legendToday = `${formatWhAndKWh(todayWh)}${formatSolarLegendDelta(todayWh, yesterdayWh, deltaPct)}`;
  const legendYesterday = formatWhAndKWh(yesterdayWh);

  if (Platform.OS === 'web') {
    const webWidth = Math.max(300, width);
    const chartPoints = buildPoints(seriesW, webWidth, WEB_CHART_HEIGHT, maxVal);
    const yesterdayChartPoints = buildPoints(yesterdaySeriesW, webWidth, WEB_CHART_HEIGHT, maxVal);
    const activeIndex =
      selectedIndex === null ? null : clamp(selectedIndex, 0, Math.max(0, points - 1));
    const selectedX = activeIndex === null ? null : bucketCenterX(activeIndex, webWidth, points);
    const d = buildSvgSmoothPath(chartPoints);
    const yesterdayD = buildDashedSvgPath(yesterdayChartPoints);
    const areaD = d
      ? `${d} L ${webWidth - PAD_X} ${WEB_CHART_HEIGHT - PAD_Y} L ${PAD_X} ${WEB_CHART_HEIGHT - PAD_Y} Z`
      : '';
    const horizontalGrid = [0, 0.5, 1].map((p) => PAD_Y + p * (WEB_CHART_HEIGHT - PAD_Y * 2));
    const verticalGrid = X_AXIS_TICKS.map((tick) => PAD_X + tick.fraction * (webWidth - PAD_X * 2));

    return (
      <YStack
        borderRadius="$4"
        borderWidth={1}
        style={{
          borderColor: semantics.chartFrameBorder,
          backgroundColor: semantics.chartFrameBackground
        }}
        overflow="hidden"
      >
        <YStack alignItems="flex-end" paddingHorizontal="$3" paddingTop="$3" paddingBottom="$1" gap="$1">
          <XStack gap="$2" alignItems="center">
            <LegendLine dotted color={semantics.chartSolarMuted} />
            <Text fontSize="$1" opacity={0.72}>
              Yesterday: {legendYesterday}
            </Text>
          </XStack>
          <XStack gap="$2" alignItems="center">
            <LegendLine color={semantics.chartSolar} />
            <Text fontSize="$1" opacity={0.9}>
              Today: {legendToday}
            </Text>
          </XStack>
        </YStack>
        <XStack padding="$2" paddingBottom="$1" alignItems="flex-start">
          <YStack width={Y_AXIS_WIDTH} height={WEB_CHART_HEIGHT} justifyContent="space-between" paddingTop="$1">
            {yAxisLabels.map((value, idx) => (
              <Text key={`y-axis-${idx}`} fontSize="$1" opacity={0.62}>
                {formatAxisW(value)}
              </Text>
            ))}
          </YStack>
          <YStack flex={1} minWidth={0}>
            <View
              onLayout={(event) => {
                setWidth(Math.round(event.nativeEvent.layout.width));
              }}
              style={{ width: '100%', height: WEB_CHART_HEIGHT, position: 'relative' }}
            >
              {width > 0 ? (
                <svg
                  width={webWidth}
                  height={WEB_CHART_HEIGHT}
                  viewBox={`0 0 ${webWidth} ${WEB_CHART_HEIGHT}`}
                  onMouseMove={(event) => {
                    setSelectedIndex(bucketIndexFromLocationX(event.nativeEvent.offsetX, webWidth, points));
                  }}
                  onMouseLeave={() => {
                    setSelectedIndex(null);
                  }}
                >
                  <defs>
                    <linearGradient id="solar-generated-grad" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={semantics.chartSolar} stopOpacity="0.24" />
                      <stop offset="100%" stopColor={semantics.chartSolar} stopOpacity="0.02" />
                    </linearGradient>
                  </defs>
                  {horizontalGrid.map((y, idx) => (
                    <line
                      key={`h-grid-${idx}`}
                      x1={PAD_X}
                      y1={y}
                      x2={webWidth - PAD_X}
                      y2={y}
                      stroke={semantics.chartGridMajor}
                      strokeWidth="1"
                    />
                  ))}
                  {verticalGrid.map((x, idx) => (
                    <line
                      key={`v-grid-${idx}`}
                      x1={x}
                      y1={PAD_Y}
                      x2={x}
                      y2={WEB_CHART_HEIGHT - PAD_Y}
                      stroke={semantics.chartGridMinor}
                      strokeWidth="1"
                    />
                  ))}
                  {areaD ? <path d={areaD} fill="url(#solar-generated-grad)" /> : null}
                  {yesterdayD ? (
                    <path
                      d={yesterdayD}
                      fill="none"
                      stroke={semantics.chartSolar}
                      strokeOpacity="0.72"
                      strokeWidth="1.4"
                      strokeLinecap="round"
                    />
                  ) : null}
                  {d ? (
                    <path d={d} fill="none" stroke={semantics.chartSolar} strokeWidth="2.4" strokeLinecap="round" />
                  ) : null}
                </svg>
              ) : null}
              {activeIndex !== null && selectedX !== null ? (
                <SelectionOverlay
                  width={webWidth}
                  height={WEB_CHART_HEIGHT}
                  selectedX={selectedX}
                  todayPoint={chartPoints[activeIndex]}
                  yesterdayPoint={yesterdayChartPoints[activeIndex]}
                  bucketLabel={formatBucketRangeLabel(activeIndex)}
                  todayBucketWh={seriesBucketWh[activeIndex] ?? 0}
                  todayBucketW={seriesW[activeIndex] ?? 0}
                  yesterdayBucketWh={yesterdaySeriesBucketWh[activeIndex] ?? 0}
                  yesterdayBucketW={yesterdaySeriesW[activeIndex] ?? 0}
                  colors={{
                    crosshair: semantics.chartSolarCrosshair,
                    yesterdaySeries: semantics.chartSolarMuted,
                    selectionRingSoft: semantics.chartSelectionRingSoft,
                    selectionRingStrong: semantics.chartSelectionRingStrong,
                    solarSeries: semantics.chartSolar,
                    tooltipBackground: semantics.tooltipBackground,
                    tooltipBorder: semantics.tooltipBorder,
                    tooltipTitle: semantics.tooltipTitle,
                    tooltipToday: semantics.tooltipToday,
                    tooltipYesterday: semantics.tooltipYesterday,
                    tooltipUp: semantics.tooltipUp,
                    tooltipDown: semantics.tooltipDown
                  }}
                />
              ) : null}
            </View>
            <View style={{ position: 'relative', height: 18 }}>
              {X_AXIS_TICKS.map((tick) => {
                const left = clamp(
                  PAD_X + tick.fraction * (webWidth - PAD_X * 2) - X_AXIS_LABEL_WIDTH / 2,
                  0,
                  webWidth - X_AXIS_LABEL_WIDTH
                );
                return (
                  <Text
                    key={`x-axis-${tick.label}`}
                    fontSize="$1"
                    opacity={0.62}
                    style={{
                      position: 'absolute',
                      left,
                      width: X_AXIS_LABEL_WIDTH,
                      textAlign: 'center'
                    }}
                  >
                    {tick.label}
                  </Text>
                );
              })}
            </View>
          </YStack>
        </XStack>
      </YStack>
    );
  }

  const chartPoints = buildPoints(seriesW, Math.max(width, 1), CHART_HEIGHT, maxVal);
  const yesterdayChartPoints = buildPoints(yesterdaySeriesW, Math.max(width, 1), CHART_HEIGHT, maxVal);
  const activeIndex = selectedIndex === null ? null : clamp(selectedIndex, 0, Math.max(0, points - 1));
  const selectedX = activeIndex === null ? null : bucketCenterX(activeIndex, Math.max(width, 1), points);
  const path = buildSkiaSmoothPath(chartPoints);
  const yesterdayPath = buildDashedSkiaPath(yesterdayChartPoints);
  const horizontalGridY = [0, 0.5, 1].map((p) => PAD_Y + p * (CHART_HEIGHT - PAD_Y * 2));
  const verticalGridX = X_AXIS_TICKS.map(
    (tick) => PAD_X + tick.fraction * (Math.max(width, 1) - PAD_X * 2)
  );

  return (
    <YStack
      borderRadius="$4"
      borderWidth={1}
      style={{
        borderColor: semantics.chartFrameBorder,
        backgroundColor: semantics.chartFrameBackground
      }}
      overflow="hidden"
    >
      <YStack alignItems="flex-end" paddingHorizontal="$3" paddingTop="$3" paddingBottom="$1" gap="$1">
        <XStack gap="$2" alignItems="center">
          <LegendLine dotted color={semantics.chartSolarMuted} />
          <Text fontSize="$1" opacity={0.72}>
            Yesterday: {legendYesterday}
          </Text>
        </XStack>
        <XStack gap="$2" alignItems="center">
          <LegendLine color={semantics.chartSolar} />
          <Text fontSize="$1" opacity={0.9}>
            Today: {legendToday}
          </Text>
        </XStack>
      </YStack>
      <XStack padding="$2" paddingBottom="$1" alignItems="flex-start">
        <YStack width={Y_AXIS_WIDTH} height={CHART_HEIGHT} justifyContent="space-between" paddingTop="$1">
          {yAxisLabels.map((value, idx) => (
            <Text key={`y-axis-native-${idx}`} fontSize="$1" opacity={0.62}>
              {formatAxisW(value)}
            </Text>
          ))}
        </YStack>
        <YStack
          flex={1}
          minWidth={0}
          onLayout={(event) => {
            setWidth(Math.round(event.nativeEvent.layout.width));
          }}
        >
          {width > 0 ? (
            <View style={{ width, height: CHART_HEIGHT, position: 'relative' }}>
              <Canvas style={{ width, height: CHART_HEIGHT }}>
                {horizontalGridY.map((y, idx) => {
                  const grid = Skia.Path.Make();
                  grid.moveTo(PAD_X, y);
                  grid.lineTo(width - PAD_X, y);
                  return (
                    <Path
                      key={`h-grid-native-${idx}`}
                      path={grid}
                      color={semantics.chartGridMajor}
                      style="stroke"
                      strokeWidth={1}
                    />
                  );
                })}
                {verticalGridX.map((x, idx) => {
                  const grid = Skia.Path.Make();
                  grid.moveTo(x, PAD_Y);
                  grid.lineTo(x, CHART_HEIGHT - PAD_Y);
                  return (
                    <Path
                      key={`v-grid-native-${idx}`}
                      path={grid}
                      color={semantics.chartGridMinor}
                      style="stroke"
                      strokeWidth={1}
                    />
                  );
                })}
                {yesterdayPath ? (
                  <Path
                    path={yesterdayPath}
                    color={semantics.chartSolarMuted}
                    style="stroke"
                    strokeWidth={1.4}
                    strokeCap="round"
                    strokeJoin="round"
                  />
                ) : null}
                {path ? (
                  <Path
                    path={path}
                    color={semantics.chartSolar}
                    style="stroke"
                    strokeWidth={2.4}
                    strokeCap="round"
                    strokeJoin="round"
                  />
                ) : null}
              </Canvas>
              {activeIndex !== null && selectedX !== null ? (
                <SelectionOverlay
                  width={width}
                  height={CHART_HEIGHT}
                  selectedX={selectedX}
                  todayPoint={chartPoints[activeIndex]}
                  yesterdayPoint={yesterdayChartPoints[activeIndex]}
                  bucketLabel={formatBucketRangeLabel(activeIndex)}
                  todayBucketWh={seriesBucketWh[activeIndex] ?? 0}
                  todayBucketW={seriesW[activeIndex] ?? 0}
                  yesterdayBucketWh={yesterdaySeriesBucketWh[activeIndex] ?? 0}
                  yesterdayBucketW={yesterdaySeriesW[activeIndex] ?? 0}
                  colors={{
                    crosshair: semantics.chartSolarCrosshair,
                    yesterdaySeries: semantics.chartSolarMuted,
                    selectionRingSoft: semantics.chartSelectionRingSoft,
                    selectionRingStrong: semantics.chartSelectionRingStrong,
                    solarSeries: semantics.chartSolar,
                    tooltipBackground: semantics.tooltipBackground,
                    tooltipBorder: semantics.tooltipBorder,
                    tooltipTitle: semantics.tooltipTitle,
                    tooltipToday: semantics.tooltipToday,
                    tooltipYesterday: semantics.tooltipYesterday,
                    tooltipUp: semantics.tooltipUp,
                    tooltipDown: semantics.tooltipDown
                  }}
                />
              ) : null}
              <Pressable
                style={{ position: 'absolute', top: 0, right: 0, bottom: 0, left: 0 }}
                onPress={(event) => {
                  setSelectedIndex(bucketIndexFromLocationX(event.nativeEvent.locationX, width, points));
                }}
              />
            </View>
          ) : (
            <Text opacity={0.6} textAlign="center">
              No solar data yet
            </Text>
          )}
          <View style={{ position: 'relative', height: 18 }}>
            {X_AXIS_TICKS.map((tick) => {
              const left = clamp(
                PAD_X + tick.fraction * (Math.max(width, 1) - PAD_X * 2) - X_AXIS_LABEL_WIDTH / 2,
                0,
                Math.max(width, 1) - X_AXIS_LABEL_WIDTH
              );
              return (
                <Text
                  key={`x-axis-native-${tick.label}`}
                  fontSize="$1"
                  opacity={0.62}
                  style={{
                    position: 'absolute',
                    left,
                    width: X_AXIS_LABEL_WIDTH,
                    textAlign: 'center'
                  }}
                >
                  {tick.label}
                </Text>
              );
            })}
          </View>
        </YStack>
      </XStack>
    </YStack>
  );
}
