import { createTamagui, createTokens } from 'tamagui';

const tokens = createTokens({
  color: {
    bg: '#f5f5f7',
    bgElevated: '#ffffff',
    bgDark: '#08090a',
    bgDarkElevated: '#111214',
    text: '#111216',
    textMuted: '#61646c',
    textDark: '#f4f6f8',
    textDarkMuted: '#9aa1ad',
    accent: '#0a84ff',
    success: '#30d158',
    warning: '#ff9f0a',
    danger: '#ff453a',
    border: '#d8dae0',
    borderDark: '#26282d'
  },
  space: {
    0: 0,
    1: 4,
    2: 8,
    3: 12,
    4: 16,
    5: 20,
    6: 24,
    7: 32,
    8: 40,
    9: 48,
    true: 16
  },
  size: {
    0: 0,
    1: 12,
    2: 14,
    3: 16,
    4: 18,
    5: 20,
    6: 24,
    7: 28,
    8: 34,
    9: 40,
    true: 16
  },
  radius: {
    0: 0,
    1: 8,
    2: 12,
    3: 16,
    4: 20,
    5: 28,
    true: 12
  },
  zIndex: {
    0: 0,
    1: 100,
    2: 200,
    3: 300
  }
});

const themes = {
  light: {
    background: tokens.color.bg,
    backgroundHover: '#ebedf1',
    backgroundPress: '#dfe2e8',
    backgroundFocus: '#d9dce4',
    color: tokens.color.text,
    colorHover: '#000000',
    colorPress: '#1d1f23',
    colorFocus: '#111216',
    borderColor: tokens.color.border,
    shadowColor: '#000000',
    accentColor: tokens.color.accent
  },
  dark: {
    background: tokens.color.bgDark,
    backgroundHover: '#121418',
    backgroundPress: '#1a1d22',
    backgroundFocus: '#1b1e24',
    color: tokens.color.textDark,
    colorHover: '#ffffff',
    colorPress: '#e9edf2',
    colorFocus: '#f2f6fb',
    borderColor: tokens.color.borderDark,
    shadowColor: '#000000',
    accentColor: tokens.color.accent
  }
};

const config = createTamagui({
  tokens,
  themes,
  settings: {
    shouldAddPrefersColorThemes: true,
    allowedStyleValues: 'somewhat-strict-web'
  },
  defaultTheme: 'light'
});

export type AppTamaguiConfig = typeof config;

declare module 'tamagui' {
  interface TamaguiCustomConfig extends AppTamaguiConfig {}
}

export default config;
