import { useMemo, useState } from 'react';
import { Platform, View } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { Canvas, Path, Skia } from '@shopify/react-native-skia';
import { SparklineTrend } from '@/shared/ui/SparklineTrend';
import type { TrendChartStyle } from '@/shared/ui/chartPrefs';

type SeriesConfig = {
  key: 'solar' | 'ac' | 'dc' | 'load';
  label: string;
  color: string;
  values: number[];
};

const CHART_HEIGHT = 170;
const PAD_X = 8;
const PAD_Y = 14;
const WEB_CHART_HEIGHT = 210;
const Y_AXIS_WIDTH = 44;
const X_AXIS_TICKS = 5;

type Point = { x: number; y: number };

function pointAt(points: Point[], idx: number): Point {
  const clamped = Math.max(0, Math.min(points.length - 1, idx));
  return points[clamped] ?? { x: 0, y: 0 };
}

function buildPoints(values: number[], width: number, height: number, min: number, max: number): Point[] {
  if (values.length < 2 || width <= 0 || height <= 0) return [];
  const plotW = Math.max(1, width - PAD_X * 2);
  const plotH = Math.max(1, height - PAD_Y * 2);
  const range = Math.max(1e-9, max - min);
  return values.map((value, idx) => ({
    x: PAD_X + (idx / (values.length - 1)) * plotW,
    y: height - PAD_Y - ((value - min) / range) * plotH
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

function formatAxisWatts(value: number): string {
  const abs = Math.abs(value);
  if (abs >= 1000) {
    return `${(value / 1000).toFixed(1)}kW`;
  }
  return `${Math.round(value)}W`;
}

function formatAgoSeconds(seconds: number): string {
  if (seconds <= 0) return 'now';
  if (seconds >= 60) return `${Math.round(seconds / 60)}m`;
  return `${Math.round(seconds)}s`;
}

function buildXAxisLabels(points: number, bucketSeconds: number): string[] {
  const totalSeconds = Math.max(0, (points - 1) * bucketSeconds);
  return Array.from({ length: X_AXIS_TICKS }, (_, idx) => {
    const fraction = 1 - idx / (X_AXIS_TICKS - 1);
    const secondsAgo = totalSeconds * fraction;
    return formatAgoSeconds(secondsAgo);
  });
}

export function PowerTrendChart({
  solar,
  ac,
  dc,
  load,
  points = 60,
  bucketSeconds = 5,
  style = 'premium'
}: {
  solar: number[];
  ac: number[];
  dc: number[];
  load: number[];
  points?: number;
  bucketSeconds?: number;
  style?: TrendChartStyle;
}) {
  const [width, setWidth] = useState(0);
  const [visible, setVisible] = useState<Record<SeriesConfig['key'], boolean>>({
    solar: true,
    ac: true,
    dc: true,
    load: true
  });
  const series = useMemo<SeriesConfig[]>(
    () => [
      { key: 'solar', label: 'Solar', color: '#ff9f0a', values: solar.slice(-points) },
      { key: 'ac', label: 'AC In', color: '#0a84ff', values: ac.slice(-points) },
      { key: 'dc', label: 'DC', color: '#bf5af2', values: dc.slice(-points) },
      { key: 'load', label: 'Load', color: '#ff453a', values: load.slice(-points) }
    ],
    [ac, dc, load, points, solar]
  );
  const activeSeries = useMemo(
    () => series.filter((s) => visible[s.key]),
    [series, visible]
  );

  const allValues = useMemo(
    () =>
      activeSeries.flatMap((s) => s.values).filter((v) => Number.isFinite(v)),
    [activeSeries]
  );
  const minVal = allValues.length ? Math.min(0, ...allValues) : 0;
  const maxVal = allValues.length ? Math.max(1, ...allValues) : 1;
  const yAxisLabels = useMemo(() => [maxVal, (maxVal + minVal) / 2, minVal], [maxVal, minVal]);
  const xAxisLabels = useMemo(() => buildXAxisLabels(points, bucketSeconds), [points, bucketSeconds]);

  if (style === 'ascii') {
    return (
      <YStack gap="$2">
        <XStack gap="$3" flexWrap="wrap">
          {series.map((s) => (
            <XStack key={s.key} alignItems="center" gap="$2">
              <View
                style={{
                  width: 10,
                  height: 10,
                  borderRadius: 5,
                  backgroundColor: s.color
                }}
              />
              <Text fontSize="$2" opacity={0.78}>
                {s.label}
              </Text>
            </XStack>
          ))}
        </XStack>
        <YStack gap="$2">
          {series.map((s) => (
            <YStack key={s.key} gap="$1">
              <Text fontSize="$2" opacity={0.75}>
                {s.label}
              </Text>
              <SparklineTrend values={s.values} points={points} />
            </YStack>
          ))}
        </YStack>
      </YStack>
    );
  }

  if (Platform.OS === 'web') {
    const webWidth = Math.max(300, width);
    const horizontalGrid = [0, 0.5, 1].map((p) => PAD_Y + p * (WEB_CHART_HEIGHT - PAD_Y * 2));
    const verticalGrid = [0, 0.25, 0.5, 0.75, 1].map((p) => PAD_X + p * (webWidth - PAD_X * 2));

    return (
      <YStack gap="$2">
        <XStack gap="$3" flexWrap="wrap">
          {series.map((s) => (
            <XStack
              key={s.key}
              alignItems="center"
              gap="$2"
              opacity={visible[s.key] ? 1 : 0.38}
              onPress={() => {
                setVisible((prev) => ({ ...prev, [s.key]: !prev[s.key] }));
              }}
              cursor="pointer"
            >
              <View
                style={{
                  width: 10,
                  height: 10,
                  borderRadius: 5,
                  backgroundColor: s.color
                }}
              />
              <Text fontSize="$2" opacity={0.78}>
                {s.label}
              </Text>
            </XStack>
          ))}
        </XStack>
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
                <Text key={`y-axis-web-${idx}`} fontSize="$1" opacity={0.62}>
                  {formatAxisWatts(value)}
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
                    {activeSeries.map((s) => {
                      const pointsList = buildPoints(s.values, webWidth, WEB_CHART_HEIGHT, minVal, maxVal);
                      const d = buildSvgSmoothPath(pointsList);
                      if (!d) return null;
                      return (
                        <path
                          key={`line-${s.key}`}
                          d={d}
                          fill="none"
                          stroke={s.color}
                          strokeWidth="2.2"
                          strokeLinecap="round"
                          strokeLinejoin="round"
                        />
                      );
                    })}
                  </svg>
                ) : null}
              </View>
              <XStack justifyContent="space-between" paddingHorizontal="$2">
                {xAxisLabels.map((label, idx) => (
                  <Text key={`x-axis-web-${idx}`} fontSize="$1" opacity={0.62}>
                    {label}
                  </Text>
                ))}
              </XStack>
            </YStack>
          </XStack>
        </YStack>
      </YStack>
    );
  }

  return (
    <YStack gap="$2">
      <XStack gap="$3" flexWrap="wrap">
        {series.map((s) => (
          <XStack
            key={s.key}
            alignItems="center"
            gap="$2"
            opacity={visible[s.key] ? 1 : 0.38}
            onPress={() => {
              setVisible((prev) => ({ ...prev, [s.key]: !prev[s.key] }));
            }}
          >
            <View
              style={{
                width: 10,
                height: 10,
                borderRadius: 5,
                backgroundColor: s.color
              }}
            />
            <Text fontSize="$2" opacity={0.78}>
              {s.label}
            </Text>
          </XStack>
        ))}
      </XStack>
      <View
        onLayout={(event) => {
          setWidth(Math.round(event.nativeEvent.layout.width));
        }}
        style={{ width: '100%' }}
      >
        <XStack
          borderRadius="$4"
          borderWidth={1}
          borderColor="rgba(255,159,10,0.18)"
          backgroundColor="rgba(255,159,10,0.04)"
          overflow="hidden"
          padding="$2"
          paddingBottom="$1"
          alignItems="flex-start"
        >
          <YStack width={Y_AXIS_WIDTH} height={CHART_HEIGHT} justifyContent="space-between" paddingTop="$1">
            {yAxisLabels.map((value, idx) => (
              <Text key={`y-axis-native-${idx}`} fontSize="$1" opacity={0.62}>
                {formatAxisWatts(value)}
              </Text>
            ))}
          </YStack>
          <YStack flex={1} minWidth={0}>
            {width > 0 ? (
              <Canvas style={{ width, height: CHART_HEIGHT }}>
                {[0, 0.5, 1].map((p, idx) => {
                  const y = PAD_Y + p * (CHART_HEIGHT - PAD_Y * 2);
                  const line = Skia.Path.Make();
                  line.moveTo(PAD_X, y);
                  line.lineTo(width - PAD_X, y);
                  return (
                    <Path
                      key={`h-grid-native-${idx}`}
                      path={line}
                      color="rgba(255,255,255,0.07)"
                      style="stroke"
                      strokeWidth={1}
                    />
                  );
                })}
                {[0, 0.25, 0.5, 0.75, 1].map((p, idx) => {
                  const x = PAD_X + p * (width - PAD_X * 2);
                  const line = Skia.Path.Make();
                  line.moveTo(x, PAD_Y);
                  line.lineTo(x, CHART_HEIGHT - PAD_Y);
                  return (
                    <Path
                      key={`v-grid-native-${idx}`}
                      path={line}
                      color="rgba(255,255,255,0.045)"
                      style="stroke"
                      strokeWidth={1}
                    />
                  );
                })}
                {activeSeries.map((s) => {
                  const pointsList = buildPoints(s.values, width, CHART_HEIGHT, minVal, maxVal);
                  const path = buildSkiaSmoothPath(pointsList);
                  if (!path) return null;
                  return (
                    <Path
                      key={s.key}
                      path={path}
                      color={s.color}
                      style="stroke"
                      strokeWidth={2}
                      strokeCap="round"
                      strokeJoin="round"
                    />
                  );
                })}
              </Canvas>
            ) : null}
            <XStack justifyContent="space-between" paddingHorizontal="$2">
              {xAxisLabels.map((label, idx) => (
                <Text key={`x-axis-native-${idx}`} fontSize="$1" opacity={0.62}>
                  {label}
                </Text>
              ))}
            </XStack>
          </YStack>
        </XStack>
      </View>
    </YStack>
  );
}
