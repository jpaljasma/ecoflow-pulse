import { useMemo, useState } from 'react';
import { Platform, View } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { Canvas, Path, Skia } from '@shopify/react-native-skia';

const CHART_HEIGHT = 170;
const WEB_CHART_HEIGHT = 210;
const PAD_X = 8;
const PAD_Y = 14;
const SOLAR_COLOR = '#ff9f0a';
const EPSILON = 1e-6;
const Y_AXIS_WIDTH = 44;
const X_AXIS_TICKS = ['06', '09', '12', '15', '18'] as const;
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

export function SolarGeneratedChart({
  valuesWh,
  points = 72
}: {
  valuesWh: number[] | undefined;
  points?: number;
}) {
  const [width, setWidth] = useState(0);
  const seriesW = useMemo(() => {
    const trimmed = (valuesWh ?? []).slice(-points).map((v) => Math.max(0, v));
    const padded =
      trimmed.length >= points
        ? trimmed
        : [...Array.from({ length: points - trimmed.length }, () => 0), ...trimmed];
    return toBucketSeries(padded).map(toWattsFromWhPerBucket);
  }, [points, valuesWh]);
  const maxVal = useMemo(
    () => roundAxisMaxWatts(seriesW.reduce((acc, value) => Math.max(acc, value), 0)),
    [seriesW]
  );
  const yAxisLabels = useMemo(() => [maxVal, maxVal / 2, 0], [maxVal]);

  if (Platform.OS === 'web') {
    const webWidth = Math.max(300, width);
    const chartPoints = buildPoints(seriesW, webWidth, WEB_CHART_HEIGHT, maxVal);
    const d = buildSvgSmoothPath(chartPoints);
    const areaD = d
      ? `${d} L ${webWidth - PAD_X} ${WEB_CHART_HEIGHT - PAD_Y} L ${PAD_X} ${WEB_CHART_HEIGHT - PAD_Y} Z`
      : '';
    const horizontalGrid = [0, 0.5, 1].map((p) => PAD_Y + p * (WEB_CHART_HEIGHT - PAD_Y * 2));
    const verticalGrid = [0, 0.25, 0.5, 0.75, 1].map((p) => PAD_X + p * (webWidth - PAD_X * 2));

    return (
      <YStack
        borderRadius="$4"
        borderWidth={1}
        borderColor="rgba(255,159,10,0.18)"
        backgroundColor="rgba(255,159,10,0.04)"
        overflow="hidden"
      >
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
              style={{ width: '100%', height: WEB_CHART_HEIGHT }}
            >
              {width > 0 ? (
                <svg width={webWidth} height={WEB_CHART_HEIGHT} viewBox={`0 0 ${webWidth} ${WEB_CHART_HEIGHT}`}>
                  <defs>
                    <linearGradient id="solar-generated-grad" x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={SOLAR_COLOR} stopOpacity="0.24" />
                      <stop offset="100%" stopColor={SOLAR_COLOR} stopOpacity="0.02" />
                    </linearGradient>
                  </defs>
                  {horizontalGrid.map((y, idx) => (
                    <line
                      key={`h-grid-${idx}`}
                      x1={PAD_X}
                      y1={y}
                      x2={webWidth - PAD_X}
                      y2={y}
                      stroke="rgba(255,255,255,0.07)"
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
                      stroke="rgba(255,255,255,0.045)"
                      strokeWidth="1"
                    />
                  ))}
                  {areaD ? <path d={areaD} fill="url(#solar-generated-grad)" /> : null}
                  {d ? (
                    <path d={d} fill="none" stroke={SOLAR_COLOR} strokeWidth="2.4" strokeLinecap="round" />
                  ) : null}
                </svg>
              ) : null}
            </View>
            <XStack justifyContent="space-between" paddingHorizontal="$2">
              {X_AXIS_TICKS.map((label) => (
                <Text key={`x-axis-${label}`} fontSize="$1" opacity={0.62}>
                  {label}:00
                </Text>
              ))}
            </XStack>
          </YStack>
        </XStack>
      </YStack>
    );
  }

  const chartPoints = buildPoints(seriesW, Math.max(width, 1), CHART_HEIGHT, maxVal);
  const path = buildSkiaSmoothPath(chartPoints);
  const horizontalGridY = [0, 0.5, 1].map((p) => PAD_Y + p * (CHART_HEIGHT - PAD_Y * 2));
  const verticalGridX = [0, 0.25, 0.5, 0.75, 1].map((p) => PAD_X + p * (Math.max(width, 1) - PAD_X * 2));

  return (
    <YStack
      borderRadius="$4"
      borderWidth={1}
      borderColor="rgba(255,159,10,0.18)"
      backgroundColor="rgba(255,159,10,0.04)"
      overflow="hidden"
    >
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
            <Canvas style={{ width, height: CHART_HEIGHT }}>
              {horizontalGridY.map((y, idx) => {
                const grid = Skia.Path.Make();
                grid.moveTo(PAD_X, y);
                grid.lineTo(width - PAD_X, y);
                return (
                  <Path
                    key={`h-grid-native-${idx}`}
                    path={grid}
                    color="rgba(255,255,255,0.07)"
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
                    color="rgba(255,255,255,0.045)"
                    style="stroke"
                    strokeWidth={1}
                  />
                );
              })}
              {path ? (
                <Path
                  path={path}
                  color={SOLAR_COLOR}
                  style="stroke"
                  strokeWidth={2.4}
                  strokeCap="round"
                  strokeJoin="round"
                />
              ) : null}
            </Canvas>
          ) : (
            <Text opacity={0.6} textAlign="center">
              No solar data yet
            </Text>
          )}
          <XStack justifyContent="space-between" paddingHorizontal="$2">
            {X_AXIS_TICKS.map((label) => (
              <Text key={`x-axis-native-${label}`} fontSize="$1" opacity={0.62}>
                {label}:00
              </Text>
            ))}
          </XStack>
        </YStack>
      </XStack>
    </YStack>
  );
}
