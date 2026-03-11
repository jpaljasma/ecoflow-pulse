import themeDefinitions from '../../../theme-definitions.json';

export type ThemeVariant = keyof typeof themeDefinitions.themes;
export type ThemeFamily = 'original' | 'new';
export type ThemeMode = 'light' | 'dark';

export type ThemeSpec = {
  label: string;
  family: ThemeFamily;
  mode: ThemeMode;
  colors: {
    background: string;
    backgroundHover: string;
    backgroundPress: string;
    backgroundFocus: string;
    backgroundElevated: string;
    color: string;
    colorHover: string;
    colorPress: string;
    colorFocus: string;
    colorMuted: string;
    borderColor: string;
    shadowColor: string;
    accentColor: string;
  };
  semantic: {
    success: string;
    warning: string;
    danger: string;
    info: string;
    solar: string;
    ac: string;
    dc: string;
    load: string;
    co2e: string;
    air: string;
    evMiles: string;
    trees: string;
    metricCold: string;
  };
};

export const themeCatalog = themeDefinitions.themes as Record<ThemeVariant, ThemeSpec>;
export const defaultThemeFamily = themeDefinitions.defaultFamily as ThemeFamily;
export const defaultThemeVariant = themeDefinitions.defaultVariant as ThemeVariant;
export const appMetadata = themeDefinitions.metadata;

export const themeFamilyOptions: Array<{
  value: ThemeFamily;
  label: string;
  description: string;
  lightPreview: ThemeSpec;
  darkPreview: ThemeSpec;
}> = [
  {
    value: 'original',
    label: 'Original',
    description: 'Keep the current Pulse palette and let your device handle light and dark mode.',
    lightPreview: themeCatalog['original-light'],
    darkPreview: themeCatalog['original-dark']
  },
  {
    value: 'new',
    label: 'New',
    description: 'Use the new mint energy palette while still following the system appearance.',
    lightPreview: themeCatalog['new-light'],
    darkPreview: themeCatalog['new-dark']
  }
];

export function getThemeSpec(variant: ThemeVariant): ThemeSpec {
  return themeCatalog[variant] ?? themeCatalog[defaultThemeVariant];
}

export function isDarkTheme(variant: ThemeVariant): boolean {
  return getThemeSpec(variant).mode === 'dark';
}

export function resolveThemeMode(colorScheme: 'light' | 'dark' | 'unspecified' | null | undefined): ThemeMode {
  return colorScheme === 'light' ? 'light' : 'dark';
}

export function resolveThemeVariant(family: ThemeFamily, mode: ThemeMode): ThemeVariant {
  const resolvedVariant = `${family}-${mode}` as ThemeVariant;
  return themeCatalog[resolvedVariant] ? resolvedVariant : defaultThemeVariant;
}
