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

function buildPath(values: number[], width: number, height: number, min: number, max: number) {
  if (values.length < 2 || width <= 0 || height <= 0) return null;
  const plotW = Math.max(1, width - PAD_X * 2);
  const plotH = Math.max(1, height - PAD_Y * 2);
  const range = Math.max(1e-9, max - min);
  const path = Skia.Path.Make();

  values.forEach((value, idx) => {
    const x = PAD_X + (idx / (values.length - 1)) * plotW;
    const y = height - PAD_Y - ((value - min) / range) * plotH;
    if (idx === 0) {
      path.moveTo(x, y);
    } else {
      path.lineTo(x, y);
    }
  });

  return path;
}

function buildSvgPath(values: number[], width: number, height: number, min: number, max: number): string {
  if (values.length < 2 || width <= 0 || height <= 0) return '';
  const plotW = Math.max(1, width - PAD_X * 2);
  const plotH = Math.max(1, height - PAD_Y * 2);
  const range = Math.max(1e-9, max - min);

  return values
    .map((value, idx) => {
      const x = PAD_X + (idx / (values.length - 1)) * plotW;
      const y = height - PAD_Y - ((value - min) / range) * plotH;
      return `${idx === 0 ? 'M' : 'L'} ${x.toFixed(2)} ${y.toFixed(2)}`;
    })
    .join(' ');
}

export function PowerTrendChart({
  solar,
  ac,
  dc,
  load,
  points = 60,
  style = 'premium'
}: {
  solar: number[];
  ac: number[];
  dc: number[];
  load: number[];
  points?: number;
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
    const gridLines = [0.2, 0.4, 0.6, 0.8].map((p) => PAD_Y + p * (WEB_CHART_HEIGHT - PAD_Y * 2));

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
          <View
            onLayout={(event) => {
              setWidth(Math.round(event.nativeEvent.layout.width));
            }}
            style={{ width: '100%', height: WEB_CHART_HEIGHT }}
          >
            {width > 0 ? (
              <svg width={webWidth} height={WEB_CHART_HEIGHT} viewBox={`0 0 ${webWidth} ${WEB_CHART_HEIGHT}`}>
                <defs>
                  {series.map((s) => (
                    <linearGradient key={`g-${s.key}`} id={`grad-${s.key}`} x1="0" y1="0" x2="0" y2="1">
                      <stop offset="0%" stopColor={s.color} stopOpacity="0.22" />
                      <stop offset="100%" stopColor={s.color} stopOpacity="0.02" />
                    </linearGradient>
                  ))}
                </defs>
                {gridLines.map((y, idx) => (
                  <line
                    key={`grid-${idx}`}
                    x1={PAD_X}
                    y1={y}
                    x2={webWidth - PAD_X}
                    y2={y}
                    stroke="rgba(255,255,255,0.08)"
                    strokeWidth="1"
                  />
                ))}
                {activeSeries.map((s) => {
                  const d = buildSvgPath(s.values, webWidth, WEB_CHART_HEIGHT, minVal, maxVal);
                  if (!d) return null;
                  const areaD = `${d} L ${webWidth - PAD_X} ${WEB_CHART_HEIGHT - PAD_Y} L ${PAD_X} ${WEB_CHART_HEIGHT - PAD_Y} Z`;
                  return (
                    <g key={`line-${s.key}`}>
                      <path d={areaD} fill={`url(#grad-${s.key})`} />
                      <path d={d} fill="none" stroke={s.color} strokeWidth="2.2" strokeLinecap="round" />
                    </g>
                  );
                })}
              </svg>
            ) : null}
          </View>
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
        style={{ width: '100%', height: CHART_HEIGHT }}
      >
        {width > 0 ? (
          <Canvas style={{ width, height: CHART_HEIGHT }}>
            {activeSeries.map((s) => {
              const path = buildPath(s.values, width, CHART_HEIGHT, minVal, maxVal);
              if (!path) return null;
              return (
                <Path
                  key={s.key}
                  path={path}
                  color={s.color}
                  style="stroke"
                  strokeWidth={2}
                />
              );
            })}
          </Canvas>
        ) : null}
      </View>
    </YStack>
  );
}
