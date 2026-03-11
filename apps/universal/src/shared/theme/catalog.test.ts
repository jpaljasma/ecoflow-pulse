import { describe, expect, it } from 'vitest';
import tamaguiConfig from '../../../tamagui.config';
import {
  defaultThemeFamily,
  defaultThemeVariant,
  resolveThemeMode,
  resolveThemeVariant,
  themeCatalog,
  themeFamilyOptions
} from './catalog';

describe('theme catalog', () => {
  it('keeps the configured default family aligned with the fallback Tamagui theme', () => {
    const tamaguiDefaultTheme = (tamaguiConfig as { defaultTheme?: string }).defaultTheme;

    expect(defaultThemeFamily).toBe('new');
    expect(defaultThemeVariant).toBe('new-dark');
    expect(themeCatalog[defaultThemeVariant]).toBeDefined();
    expect(tamaguiDefaultTheme).toBe(defaultThemeVariant);
  });

  it('exposes original and new theme families with light and dark previews', () => {
    expect(themeFamilyOptions).toEqual(
      expect.arrayContaining([
        expect.objectContaining({
          value: 'original',
          lightPreview: expect.objectContaining({ family: 'original', mode: 'light' }),
          darkPreview: expect.objectContaining({ family: 'original', mode: 'dark' })
        }),
        expect.objectContaining({
          value: 'new',
          lightPreview: expect.objectContaining({ family: 'new', mode: 'light' }),
          darkPreview: expect.objectContaining({ family: 'new', mode: 'dark' })
        })
      ])
    );
  });

  it('defines semantic theme colors for status, charts, and derived dashboard badges', () => {
    for (const spec of Object.values(themeCatalog)) {
      expect(spec.semantic).toEqual(
        expect.objectContaining({
          success: expect.any(String),
          warning: expect.any(String),
          danger: expect.any(String),
          info: expect.any(String),
          solar: expect.any(String),
          ac: expect.any(String),
          dc: expect.any(String),
          load: expect.any(String),
          co2e: expect.any(String),
          air: expect.any(String),
          evMiles: expect.any(String),
          trees: expect.any(String),
          metricCold: expect.any(String)
        })
      );
    }
  });

  it('resolves the runtime variant from family and system appearance', () => {
    expect(resolveThemeMode('light')).toBe('light');
    expect(resolveThemeMode('dark')).toBe('dark');
    expect(resolveThemeMode(null)).toBe('dark');

    expect(resolveThemeVariant('new', 'light')).toBe('new-light');
    expect(resolveThemeVariant('new', 'dark')).toBe('new-dark');
    expect(resolveThemeVariant('original', 'light')).toBe('original-light');
    expect(resolveThemeVariant('original', 'dark')).toBe('original-dark');
  });
});
