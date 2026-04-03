import { useMemo, useState } from 'react';
import { Platform, View } from 'react-native';
import { Text, XStack, YStack } from 'tamagui';
import { useThemeSemantics } from '@/shared/theme/semantic';

type SeriesConfig = {
  key: 'solar' | 'grid' | 'batteryCharge' | 'load' | 'acOutput' | 'dcOutput' | 'batteryDischarge';
  label: string;
  color: string;
  values: number[];
  direction: 'positive' | 'negative';
};

type Point = { x: number; y: number };

const CHART_HEIGHT = 216;
const WEB_CHART_HEIGHT = 248;
const PAD_X = 12;
const PAD_Y = 18;
const Y_AXIS_WIDTH = 48;
const X_AXIS_TICKS = 5;

function formatAxisKWh(value: number): string {
  if (value === 0) return '0';
  if (Math.abs(value) >= 10) {
    return `${value.toFixed(0)}kWh`;
  }
  return `${value.toFixed(1)}kWh`;
}

function formatAgoSeconds(seconds: number): string {
  if (seconds <= 0) return 'now';
  if (seconds >= 86400) return `${Math.round(seconds / 86400)}d`;
  if (seconds >= 3600) return `${Math.round(seconds / 3600)}h`;
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

function buildOverlayPoints(values: number[], width: number, height: number, maxMagnitude: number): Point[] {
  if (values.length < 2 || width <= 0 || height <= 0 || maxMagnitude <= 0) return [];
  const plotW = Math.max(1, width - PAD_X * 2);
  const plotH = Math.max(1, height - PAD_Y * 2);
  const step = plotW / values.length;
  const zeroY = PAD_Y + plotH / 2;
  const halfPlot = plotH / 2;

  return values.map((value, idx) => ({
    x: PAD_X + idx * step + step / 2,
    y: zeroY - (value / maxMagnitude) * halfPlot
  }));
}

function buildOverlayPath(points: Point[]): string {
  if (points.length < 2) return '';
  const first = points[0];
  if (!first) return '';
  let d = `M ${first.x.toFixed(2)} ${first.y.toFixed(2)}`;
  for (let i = 1; i < points.length; i += 1) {
    const point = points[i];
    if (!point) continue;
    d += ` L ${point.x.toFixed(2)} ${point.y.toFixed(2)}`;
  }
  return d;
}

export function EnergyTrendChart({
  solar,
  grid,
  acOutput,
  load,
  dcOutput,
  batteryCharge,
  batteryDischarge,
  previousNet,
  points = 24,
  bucketSeconds = 3600
}: {
  solar: number[];
  grid: number[];
  acOutput: number[];
  load: number[];
  dcOutput: number[];
  batteryCharge: number[];
  batteryDischarge: number[];
  previousNet?: number[];
  points?: number;
  bucketSeconds?: number;
}) {
  const [width, setWidth] = useState(0);
  const semantics = useThemeSemantics();

  const series = useMemo<SeriesConfig[]>(
    () => [
      {
        key: 'solar',
        label: 'Solar',
        color: semantics.chartSolar,
        values: solar.slice(-points),
        direction: 'positive'
      },
      {
        key: 'grid',
        label: 'Grid in',
        color: semantics.chartAc,
        values: grid.slice(-points),
        direction: 'positive'
      },
      {
        key: 'batteryCharge',
        label: 'Charge',
        color: semantics.chartBatteryCharge,
        values: batteryCharge.slice(-points),
        direction: 'positive'
      },
      {
        key: 'load',
        label: 'Load',
        color: semantics.chartLoad,
        values: load.slice(-points),
        direction: 'negative'
      },
      {
        key: 'acOutput',
        label: 'AC out',
        color: semantics.chartAcOutput,
        values: acOutput.slice(-points),
        direction: 'negative'
      },
      {
        key: 'dcOutput',
        label: 'DC out',
        color: semantics.chartDc,
        values: dcOutput.slice(-points),
        direction: 'negative'
      },
      {
        key: 'batteryDischarge',
        label: 'Discharge',
        color: semantics.chartBatteryDischarge,
        values: batteryDischarge.slice(-points),
        direction: 'negative'
      }
    ],
    [
      acOutput,
      batteryCharge,
      batteryDischarge,
      dcOutput,
      grid,
      load,
      points,
      semantics.chartAc,
      semantics.chartAcOutput,
      semantics.chartBatteryCharge,
      semantics.chartBatteryDischarge,
      semantics.chartDc,
      semantics.chartLoad,
      semantics.chartSolar,
      solar
    ]
  );

  const positiveSeries = useMemo(() => series.filter((item) => item.direction === 'positive'), [series]);
  const negativeSeries = useMemo(() => series.filter((item) => item.direction === 'negative'), [series]);

  const pointCount = Math.max(
    0,
    ...series.map((item) => item.values.length),
    previousNet?.slice(-points).length ?? 0
  );

  const positiveTotals = useMemo(
    () =>
      Array.from({ length: pointCount }, (_, idx) =>
        positiveSeries.reduce((sum, item) => sum + Math.max(0, item.values[idx] ?? 0), 0)
      ),
    [pointCount, positiveSeries]
  );
  const negativeTotals = useMemo(
    () =>
      Array.from({ length: pointCount }, (_, idx) =>
        negativeSeries.reduce((sum, item) => sum + Math.max(0, item.values[idx] ?? 0), 0)
      ),
    [negativeSeries, pointCount]
  );

  const maxMagnitude = useMemo(() => {
    const biggest = Math.max(1, ...positiveTotals, ...negativeTotals);
    return Math.ceil(biggest * 10) / 10;
  }, [negativeTotals, positiveTotals]);

  const yAxisLabels = useMemo(
    () => [maxMagnitude, maxMagnitude / 2, 0, -(maxMagnitude / 2), -maxMagnitude],
    [maxMagnitude]
  );
  const xAxisLabels = useMemo(() => buildXAxisLabels(points, bucketSeconds), [points, bucketSeconds]);

  const renderChart = (chartWidth: number, chartHeight: number) => {
    if (!pointCount || chartWidth <= 0 || chartHeight <= 0) return null;

    const plotW = Math.max(1, chartWidth - PAD_X * 2);
    const plotH = Math.max(1, chartHeight - PAD_Y * 2);
    const zeroY = PAD_Y + plotH / 2;
    const halfPlot = plotH / 2;
    const step = plotW / pointCount;
    const barWidth = Math.max(4, Math.min(22, step * 0.66));
    const overlayValues = previousNet?.slice(-pointCount) ?? [];
    const overlayPath = buildOverlayPath(buildOverlayPoints(overlayValues, chartWidth, chartHeight, maxMagnitude));

    if (Platform.OS === 'web') {
      return (
        <svg width={chartWidth} height={chartHeight} viewBox={`0 0 ${chartWidth} ${chartHeight}`}>
          {yAxisLabels.map((value, idx) => {
            const y = zeroY - (value / maxMagnitude) * halfPlot;
            return (
              <line
                key={`energy-h-grid-${idx}`}
                x1={PAD_X}
                y1={y}
                x2={chartWidth - PAD_X}
                y2={y}
                stroke={value === 0 ? semantics.chartSelectionRingSoft : semantics.chartGridMajor}
                strokeWidth={value === 0 ? '1.4' : '1'}
              />
            );
          })}
          {[0, 0.25, 0.5, 0.75, 1].map((p, idx) => {
            const x = PAD_X + p * plotW;
            return (
              <line
                key={`energy-v-grid-${idx}`}
                x1={x}
                y1={PAD_Y}
                x2={x}
                y2={chartHeight - PAD_Y}
                stroke={semantics.chartGridMinor}
                strokeWidth="1"
              />
            );
          })}

          {Array.from({ length: pointCount }, (_, idx) => {
            const left = PAD_X + idx * step + (step - barWidth) / 2;
            let positiveCursor = zeroY;
            let negativeCursor = zeroY;

            return (
              <g key={`bar-group-${idx}`}>
                {positiveSeries.map((item) => {
                  const value = Math.max(0, item.values[idx] ?? 0);
                  if (value <= 0) return null;
                  const heightPx = (value / maxMagnitude) * halfPlot;
                  positiveCursor -= heightPx;
                  return (
                    <rect
                      key={`${item.key}-${idx}`}
                      x={left}
                      y={positiveCursor}
                      width={barWidth}
                      height={heightPx}
                      rx={Math.min(4, barWidth / 2)}
                      fill={item.color}
                    />
                  );
                })}
                {negativeSeries.map((item) => {
                  const value = Math.max(0, item.values[idx] ?? 0);
                  if (value <= 0) return null;
                  const heightPx = (value / maxMagnitude) * halfPlot;
                  const rectY = negativeCursor;
                  negativeCursor += heightPx;
                  return (
                    <rect
                      key={`${item.key}-${idx}`}
                      x={left}
                      y={rectY}
                      width={barWidth}
                      height={heightPx}
                      rx={Math.min(4, barWidth / 2)}
                      fill={item.color}
                    />
                  );
                })}
              </g>
            );
          })}

          {overlayPath ? (
            <path
              d={overlayPath}
              fill="none"
              stroke={semantics.chartCompareLine}
              strokeWidth="1.5"
              strokeLinecap="round"
              strokeLinejoin="round"
            />
          ) : null}
        </svg>
      );
    }

    const overlayPoints = buildOverlayPoints(overlayValues, chartWidth, chartHeight, maxMagnitude);

    return (
      <View style={{ width: chartWidth, height: chartHeight }}>
        {yAxisLabels.map((value, idx) => {
          const y = zeroY - (value / maxMagnitude) * halfPlot;
          return (
            <View
              key={`energy-h-grid-native-${idx}`}
              style={{
                position: 'absolute',
                left: PAD_X,
                right: PAD_X,
                top: y,
                height: value === 0 ? 1.4 : 1,
                backgroundColor: value === 0 ? semantics.chartSelectionRingSoft : semantics.chartGridMajor
              }}
            />
          );
        })}
        {[0, 0.25, 0.5, 0.75, 1].map((p, idx) => {
          const x = PAD_X + p * plotW;
          return (
            <View
              key={`energy-v-grid-native-${idx}`}
              style={{
                position: 'absolute',
                top: PAD_Y,
                bottom: PAD_Y,
                left: x,
                width: 1,
                backgroundColor: semantics.chartGridMinor
              }}
            />
          );
        })}

        {Array.from({ length: pointCount }, (_, idx) => {
          const left = PAD_X + idx * step + (step - barWidth) / 2;
          let positiveCursor = zeroY;
          let negativeCursor = zeroY;

          return (
            <View key={`native-bar-group-${idx}`}>
              {positiveSeries.map((item) => {
                const value = Math.max(0, item.values[idx] ?? 0);
                if (value <= 0) return null;
                const heightPx = (value / maxMagnitude) * halfPlot;
                positiveCursor -= heightPx;
                return (
                  <View
                    key={`${item.key}-${idx}`}
                    style={{
                      position: 'absolute',
                      left,
                      top: positiveCursor,
                      width: barWidth,
                      height: heightPx,
                      borderRadius: Math.min(4, barWidth / 2),
                      backgroundColor: item.color
                    }}
                  />
                );
              })}
              {negativeSeries.map((item) => {
                const value = Math.max(0, item.values[idx] ?? 0);
                if (value <= 0) return null;
                const heightPx = (value / maxMagnitude) * halfPlot;
                const rectY = negativeCursor;
                negativeCursor += heightPx;
                return (
                  <View
                    key={`${item.key}-${idx}`}
                    style={{
                      position: 'absolute',
                      left,
                      top: rectY,
                      width: barWidth,
                      height: heightPx,
                      borderRadius: Math.min(4, barWidth / 2),
                      backgroundColor: item.color
                    }}
                  />
                );
              })}
            </View>
          );
        })}

        {overlayPoints.map((point, idx) => {
          const next = overlayPoints[idx + 1];
          if (!next) return null;
          const dx = next.x - point.x;
          const dy = next.y - point.y;
          const length = Math.sqrt(dx * dx + dy * dy);
          const angle = (Math.atan2(dy, dx) * 180) / Math.PI;
          return (
            <View
              key={`overlay-segment-${idx}`}
              style={{
                position: 'absolute',
                left: point.x,
                top: point.y,
                width: length,
                height: 2,
                borderRadius: 999,
                backgroundColor: semantics.chartCompareLine,
                transform: [{ rotate: `${angle}deg` }],
                transformOrigin: 'left center'
              } as any}
            />
          );
        })}
      </View>
    );
  };

  const chartFrameHeight = Platform.OS === 'web' ? WEB_CHART_HEIGHT : CHART_HEIGHT;
  const chartWidth = Math.max(300, width);

  return (
    <YStack gap="$3">
      <XStack gap="$3" flexWrap="wrap">
        {series.map((item) => (
          <XStack key={item.key} alignItems="center" gap="$2" opacity={0.94}>
            <View style={{ width: 10, height: 10, borderRadius: 5, backgroundColor: item.color }} />
            <Text fontSize="$2" opacity={0.78}>
              {item.label}
            </Text>
          </XStack>
        ))}
        {previousNet?.length ? (
          <XStack alignItems="center" gap="$2" opacity={0.82}>
            <View style={{ width: 14, height: 2, borderRadius: 999, backgroundColor: semantics.chartCompareLine }} />
            <Text fontSize="$2" opacity={0.78}>
              Previous period
            </Text>
          </XStack>
        ) : null}
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
          style={{
            borderColor: semantics.chartFrameBorder,
            backgroundColor: semantics.chartFrameBackground
          }}
          overflow="hidden"
          padding="$3"
          paddingBottom="$2"
          alignItems="flex-start"
        >
          <YStack width={Y_AXIS_WIDTH} height={chartFrameHeight} justifyContent="space-between" paddingTop="$1">
            {yAxisLabels.map((value, idx) => (
              <Text key={`energy-y-axis-${idx}`} fontSize="$1" opacity={0.62}>
                {formatAxisKWh(value)}
              </Text>
            ))}
          </YStack>
          <YStack flex={1} minWidth={0}>
            {width > 0 ? renderChart(chartWidth, chartFrameHeight) : null}
            <XStack justifyContent="space-between" paddingHorizontal="$2" paddingTop="$2">
              {xAxisLabels.map((label, idx) => (
                <Text key={`energy-x-axis-${idx}`} fontSize="$1" opacity={0.62}>
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
