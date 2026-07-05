import { useMemo } from 'react';
import { useAppTheme } from './useAppTheme';

export type ConnectionStatus =
  | 'connected'
  | 'auth_required'
  | 'reconnecting'
  | 'connecting'
  | 'disconnected';

export type EnergyImpactMetricKey = 'co2e' | 'air' | 'solar' | 'evMiles' | 'trees';

export function useThemeSemantics() {
  const { spec, isDark } = useAppTheme();

  return useMemo(() => {
    const neutralBase = isDark ? spec.colors.colorMuted : spec.colors.borderColor;
    const solar = spec.semantic.solar;
    const info = spec.semantic.info;
    const elevated = spec.colors.backgroundElevated;
    const solarBadgeBase = isDark
      ? mix(elevated, solar, 0.12)
      : mix(elevated, solar, 0.06);

    return {
      mutedPanelBackground: withAlpha(neutralBase, isDark ? 0.14 : 0.12),
      mutedPanelBorder: withAlpha(neutralBase, isDark ? 0.3 : 0.26),
      surfaceRaised: isDark ? mix(elevated, '#000000', 0.08) : mix(elevated, '#ffffff', 0.3),
      surfaceRaisedBorder: withAlpha(spec.colors.borderColor, isDark ? 0.88 : 0.74),
      sectionBorder: spec.colors.borderColor,
      sectionBackground: spec.colors.backgroundHover,
      sectionBackgroundStrong: isDark ? mix(spec.colors.backgroundHover, '#ffffff', 0.02) : mix(spec.colors.backgroundHover, '#ffffff', 0.4),
      subtleText: spec.colors.colorMuted,
      subtleStrongText: mix(spec.colors.colorMuted, spec.colors.color, isDark ? 0.24 : 0.38),
      energyCardBackground: withAlpha(spec.colors.accentColor, isDark ? 0.1 : 0.08),
      energyCardBorder: withAlpha(spec.colors.accentColor, isDark ? 0.42 : 0.38),
      heroBackground: isDark
        ? `linear-gradient(135deg, ${withAlpha('#142238', 0.98)} 0%, ${withAlpha('#0f1724', 1)} 46%, ${withAlpha('#111d2d', 1)} 100%)`
        : `linear-gradient(135deg, ${withAlpha('#f9fbff', 1)} 0%, ${withAlpha('#eef4fb', 1)} 46%, ${withAlpha('#e7eef9', 1)} 100%)`,
      heroGlow: withAlpha(solar, isDark ? 0.18 : 0.1),
      heroAccent: emphasize(spec.colors.accentColor, isDark ? 0.28 : 0.06),
      heroBorder: withAlpha(spec.colors.accentColor, isDark ? 0.26 : 0.22),
      tileBackground: isDark ? mix(elevated, '#ffffff', 0.015) : mix(elevated, '#ffffff', 0.65),
      tileBorder: withAlpha(spec.colors.borderColor, isDark ? 0.74 : 0.72),
      railBackground: isDark ? mix(spec.colors.background, '#000000', 0.14) : mix(spec.colors.background, '#ffffff', 0.1),
      railBorder: withAlpha(spec.colors.borderColor, isDark ? 0.76 : 0.72),
      navBrandBackground: withAlpha(spec.colors.accentColor, isDark ? 0.16 : 0.1),
      navBrandBorder: withAlpha(spec.colors.accentColor, isDark ? 0.34 : 0.26),
      navSectionLabel: mix(spec.colors.colorMuted, spec.colors.color, isDark ? 0.2 : 0.3),
      navToggleBackground: withAlpha(neutralBase, isDark ? 0.12 : 0.08),
      navToggleBorder: withAlpha(neutralBase, isDark ? 0.3 : 0.24),
      navItemActiveBackground: withAlpha(spec.colors.accentColor, isDark ? 0.16 : 0.1),
      navItemActiveBorder: withAlpha(spec.colors.accentColor, isDark ? 0.34 : 0.24),
      navItemHoverBackground: withAlpha(spec.colors.colorMuted, isDark ? 0.1 : 0.08),
      navItemHoverBorder: withAlpha(spec.colors.borderColor, isDark ? 0.32 : 0.24),
      navItemActiveIconBackground: withAlpha(spec.colors.accentColor, isDark ? 0.22 : 0.16),
      navItemIdleIconBackground: withAlpha(neutralBase, isDark ? 0.16 : 0.1),
      navItemActiveText: mix(spec.colors.color, spec.colors.accentColor, isDark ? 0.1 : 0.18),
      navItemActiveSubtleText: mix(spec.colors.colorMuted, spec.colors.accentColor, isDark ? 0.26 : 0.3),
      navItemIdleText: spec.colors.colorMuted,
      navItemIndicator: emphasize(spec.colors.accentColor, isDark ? 0.42 : 0.18),
      energyLeafBackground: withAlpha(spec.semantic.air, isDark ? 0.18 : 0.14),
      energyLeafBorder: withAlpha(spec.semantic.air, isDark ? 0.36 : 0.3),
      energyLeafText: emphasize(spec.semantic.air, isDark),
      actionBackground: withAlpha(info, isDark ? 0.14 : 0.08),
      actionBorder: withAlpha(info, isDark ? 0.34 : 0.24),
      actionHoverBackground: withAlpha(info, isDark ? 0.22 : 0.14),
      actionPressBackground: withAlpha(info, isDark ? 0.28 : 0.18),
      actionText: emphasize(info, isDark),
      periodActiveBackground: isDark
        ? mix(spec.semantic.success, '#000000', 0.26)
        : mix(spec.semantic.success, '#000000', 0.12),
      periodActiveBorder: isDark
        ? mix(spec.semantic.success, '#ffffff', 0.12)
        : mix(spec.semantic.success, '#000000', 0.22),
      periodActiveText: isDark ? '#f4fff8' : '#f8fffb',
      periodIdleBackground: withAlpha(neutralBase, isDark ? 0.12 : 0.08),
      periodIdleBorder: withAlpha(neutralBase, isDark ? 0.36 : 0.34),
      periodIdleText: isDark ? spec.colors.colorMuted : mix(spec.colors.color, spec.colors.colorMuted, 0.18),
      solarBadgeBorder: withAlpha(solar, isDark ? 0.52 : 0.5),
      solarBadgeBackground: solarBadgeBase,
      solarBadgeGradientStart: withAlpha(solar, isDark ? 0.14 : 0.12),
      solarBadgeGradientEnd: withAlpha(solar, isDark ? 0.03 : 0.02),
      solarBadgeTitle: isDark ? mix(solar, '#ffffff', 0.3) : mix(solar, '#000000', 0.26),
      solarBadgeValue: isDark ? mix(spec.colors.color, solar, 0.08) : mix(spec.colors.color, solar, 0.16),
      solarBadgeDelta: isDark ? mix(solar, '#ffffff', 0.18) : mix(solar, '#000000', 0.16),
      statusSuccess: spec.semantic.success,
      statusWarning: spec.semantic.warning,
      statusDanger: spec.semantic.danger,
      statusSuccessBackground: withAlpha(spec.semantic.success, isDark ? 0.18 : 0.12),
      statusSuccessBorder: withAlpha(spec.semantic.success, isDark ? 0.36 : 0.26),
      statusWarningBackground: withAlpha(spec.semantic.warning, isDark ? 0.2 : 0.12),
      statusWarningBorder: withAlpha(spec.semantic.warning, isDark ? 0.4 : 0.3),
      statusDangerBackground: withAlpha(spec.semantic.danger, isDark ? 0.18 : 0.12),
      statusDangerBorder: withAlpha(spec.semantic.danger, isDark ? 0.36 : 0.26),
      metricCold: spec.semantic.metricCold,
      chartFrameBorder: withAlpha(solar, isDark ? 0.2 : 0.18),
      chartFrameBackground: withAlpha(solar, isDark ? 0.05 : 0.04),
      chartGridMajor: withAlpha(spec.colors.color, isDark ? 0.12 : 0.1),
      chartGridMinor: withAlpha(spec.colors.color, isDark ? 0.075 : 0.065),
      chartSelectionRingSoft: withAlpha(spec.colors.color, isDark ? 0.45 : 0.36),
      chartSelectionRingStrong: withAlpha(spec.colors.color, isDark ? 0.58 : 0.48),
      chartCompareLine: withAlpha(spec.colors.color, isDark ? 0.54 : 0.46),
      chartSolar: spec.semantic.solar,
      chartSolarMuted: withAlpha(spec.semantic.solar, 0.72),
      chartSolarCrosshair: withAlpha(spec.semantic.solar, isDark ? 0.32 : 0.28),
      chartAc: spec.semantic.ac,
      chartAcOutput: mix(spec.semantic.ac, spec.semantic.load, isDark ? 0.32 : 0.38),
      chartDc: spec.semantic.dc,
      chartLoad: spec.semantic.load,
      chartBatteryPower: isDark
        ? mix(spec.semantic.success, spec.semantic.solar, 0.24)
        : mix(spec.semantic.success, spec.semantic.solar, 0.16),
      batteryFlowCharge: emphasize(spec.semantic.success, isDark),
      batteryFlowDischarge: isDark
        ? mix(spec.colors.colorMuted, '#ffffff', 0.1)
        : mix(spec.colors.color, spec.colors.colorMuted, 0.22),
      chartBatteryCharge: emphasize(spec.semantic.success, isDark),
      chartBatteryDischarge: isDark
        ? mix(spec.colors.colorMuted, '#ffffff', 0.1)
        : mix(spec.colors.color, spec.colors.colorMuted, 0.22),
      metricCo2eBase: spec.semantic.co2e,
      metricAirBase: spec.semantic.air,
      metricEvMilesBase: spec.semantic.evMiles,
      metricTreesBase: spec.semantic.trees,
      tooltipBackground: withAlpha(spec.colors.backgroundElevated, isDark ? 0.98 : 0.98),
      tooltipBorder: withAlpha(spec.semantic.solar, isDark ? 0.34 : 0.3),
      tooltipTitle: mix(spec.colors.colorMuted, spec.semantic.solar, isDark ? 0.28 : 0.18),
      tooltipToday: emphasize(spec.semantic.solar, isDark),
      tooltipYesterday: isDark
        ? mix(spec.semantic.solar, '#ffffff', 0.36)
        : mix(spec.semantic.solar, '#000000', 0.42),
      tooltipUp: emphasize(spec.semantic.success, isDark),
      tooltipDown: emphasize(spec.semantic.danger, isDark)
    };
  }, [isDark, spec]);
}

export function getConnectionStatusColor(
  status: ConnectionStatus,
  semantics: ReturnType<typeof useThemeSemantics>
): string {
  if (status === 'connected') return semantics.statusSuccess;
  if (status === 'auth_required') return semantics.statusWarning;
  if (status === 'reconnecting' || status === 'connecting') return semantics.statusWarning;
  return semantics.statusDanger;
}

export function getEnergyImpactBadgeColors(
  metricKey: EnergyImpactMetricKey,
  semantics: ReturnType<typeof useThemeSemantics>
) {
  const base = getEnergyImpactBaseColor(metricKey, semantics);
  const darkSurface = isLikelyDarkTextSurface(semantics);
  return {
    bg: withAlpha(base, darkSurface ? 0.2 : 0.14),
    color: emphasize(base, darkSurface)
  };
}

function getEnergyImpactBaseColor(
  metricKey: EnergyImpactMetricKey,
  semantics: ReturnType<typeof useThemeSemantics>
): string {
  switch (metricKey) {
    case 'co2e':
      return semantics.metricCo2eBase;
    case 'air':
      return semantics.metricAirBase;
    case 'solar':
      return semantics.chartSolar;
    case 'evMiles':
      return semantics.metricEvMilesBase;
    case 'trees':
      return semantics.metricTreesBase;
  }
}

function isLikelyDarkTextSurface(semantics: ReturnType<typeof useThemeSemantics>): boolean {
  return brightness(semantics.sectionBackground) < 0.45;
}

function emphasize(hex: string, isDarkOrStrength: boolean | number): string {
  if (typeof isDarkOrStrength === 'number') {
    return mix(hex, '#ffffff', isDarkOrStrength);
  }
  return isDarkOrStrength ? mix(hex, '#ffffff', 0.4) : mix(hex, '#000000', 0.22);
}

function mix(hexA: string, hexB: string, ratio: number): string {
  const a = hexToRgb(hexA);
  const b = hexToRgb(hexB);
  const clamped = clamp(ratio, 0, 1);
  return rgbToHex({
    red: Math.round(a.red + (b.red - a.red) * clamped),
    green: Math.round(a.green + (b.green - a.green) * clamped),
    blue: Math.round(a.blue + (b.blue - a.blue) * clamped)
  });
}

function withAlpha(hex: string, alpha: number): string {
  const { red, green, blue } = hexToRgb(hex);
  return `rgba(${red}, ${green}, ${blue}, ${clamp(alpha, 0, 1)})`;
}

function brightness(hex: string): number {
  const { red, green, blue } = hexToRgb(hex);
  return (red * 299 + green * 587 + blue * 114) / 255000;
}

function hexToRgb(hex: string): { red: number; green: number; blue: number } {
  const normalized = hex.replace('#', '');
  const value =
    normalized.length === 3
      ? normalized
          .split('')
          .map((part) => part + part)
          .join('')
      : normalized;

  return {
    red: Number.parseInt(value.slice(0, 2), 16),
    green: Number.parseInt(value.slice(2, 4), 16),
    blue: Number.parseInt(value.slice(4, 6), 16)
  };
}

function rgbToHex({
  red,
  green,
  blue
}: {
  red: number;
  green: number;
  blue: number;
}): string {
  return `#${[red, green, blue]
    .map((value) => clamp(Math.round(value), 0, 255).toString(16).padStart(2, '0'))
    .join('')}`;
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value));
}
