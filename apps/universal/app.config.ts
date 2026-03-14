import type { ExpoConfig } from 'expo/config';

import themeDefinitions from './theme-definitions.json';

const defaultTheme = themeDefinitions.themes[themeDefinitions.defaultVariant as keyof typeof themeDefinitions.themes];

const config: ExpoConfig = {
  name: themeDefinitions.metadata.title,
  slug: 'ecoflow-pulse-universal',
  scheme: 'ecoflowpulse',
  version: '1.0.0',
  orientation: 'portrait',
  userInterfaceStyle: 'automatic',
  icon: './assets/icon.png',
  description: themeDefinitions.metadata.description,
  plugins: ['expo-router'],
  ios: {
    icon: './assets/icon.png'
  },
  android: {
    icon: './assets/icon.png',
    adaptiveIcon: {
      foregroundImage: './assets/adaptive-icon-foreground.png',
      backgroundColor: defaultTheme.colors.background
    }
  },
  web: {
    bundler: 'metro',
    favicon: './assets/favicon.png',
    themeColor: defaultTheme.colors.background,
    backgroundColor: defaultTheme.colors.background
  },
  extra: {
    shareImagePath: '/social-share.png',
    metadata: themeDefinitions.metadata,
    oidcIssuerUrl: process.env.EXPO_PUBLIC_OIDC_ISSUER_URL ?? '',
    oidcClientId: process.env.EXPO_PUBLIC_OIDC_CLIENT_ID ?? '',
    oidcAudience: process.env.EXPO_PUBLIC_OIDC_AUDIENCE ?? '',
    oidcScopes: process.env.EXPO_PUBLIC_OIDC_SCOPES ?? 'openid profile email offline_access'
  }
};

export default config;
