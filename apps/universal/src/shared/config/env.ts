import Constants from 'expo-constants';
import * as Linking from 'expo-linking';
import { Platform } from 'react-native';

const extra = Constants.expoConfig?.extra ?? {};
const defaultWebAssetBaseUrl =
  Platform.OS === 'web' && typeof window !== 'undefined' && window.location?.origin
    ? window.location.origin
    : '';
const defaultNativeHost = (() => {
  if (Platform.OS === 'web') return '';
  try {
    const url = Linking.createURL('/');
    const parsed = new URL(url);
    if (parsed.hostname) return parsed.hostname;
  } catch {
    // Fallback below.
  }
  const debuggerHost = (Constants as unknown as { manifest2?: { extra?: { expoGo?: { debuggerHost?: string } } } })
    .manifest2?.extra?.expoGo?.debuggerHost;
  if (typeof debuggerHost === 'string' && debuggerHost.length > 0) {
    return debuggerHost.split(':')[0] ?? '';
  }
  return '';
})();
const defaultNativeWsUrl = `ws://${defaultNativeHost || '127.0.0.1'}:8080/ws`;
const defaultNativeHttpBase = (() => {
  if (Platform.OS === 'web') return '';
  try {
    const url = Linking.createURL('/');
    const parsed = new URL(url);
    const host = parsed.hostname || defaultNativeHost || '127.0.0.1';
    const port = parsed.port || '8081';
    return `http://${host}:${port}`;
  } catch {
    const host = defaultNativeHost || '127.0.0.1';
    return `http://${host}:8081`;
  }
})();

export const env = {
  defaultAssetBaseUrl: defaultWebAssetBaseUrl,
  apiUrl:
    process.env.EXPO_PUBLIC_API_URL ??
    (typeof extra.apiUrl === 'string' ? extra.apiUrl : 'mock://ecoflow'),
  wsUrl:
    process.env.EXPO_PUBLIC_WS_URL ??
    (typeof extra.wsUrl === 'string'
      ? extra.wsUrl
      : Platform.OS === 'web'
        ? 'ws://localhost:8080/ws'
        : defaultNativeWsUrl),
  wsUrlExplicit:
    typeof process.env.EXPO_PUBLIC_WS_URL === 'string' ||
    typeof extra.wsUrl === 'string',
  mockLogUrl:
    process.env.EXPO_PUBLIC_MOCK_LOG_URL ??
    (typeof extra.mockLogUrl === 'string'
      ? extra.mockLogUrl
      : Platform.OS === 'web'
        ? '/mock/mqtt.log'
        : `${defaultNativeHttpBase}/logs/mqtt.log`),
  mockTrainingUrl:
    process.env.EXPO_PUBLIC_MOCK_TRAINING_URL ??
    (typeof extra.mockTrainingUrl === 'string'
      ? extra.mockTrainingUrl
      : Platform.OS === 'web'
        ? '/mock/telemetry_training.csv'
        : `${defaultNativeHttpBase}/logs/telemetry_training.csv`),
  assetBaseUrl:
    process.env.EXPO_PUBLIC_ASSET_BASE_URL ??
    (typeof extra.assetBaseUrl === 'string'
      ? extra.assetBaseUrl
      : defaultWebAssetBaseUrl),
  closePageTransition:
    process.env.EXPO_PUBLIC_CLOSE_PAGE_TRANSITION ??
    (typeof extra.closePageTransition === 'string'
      ? extra.closePageTransition
      : 'none'),
  closePageTransitionMs:
    Number(process.env.EXPO_PUBLIC_CLOSE_PAGE_TRANSITION_MS) ||
    (typeof extra.closePageTransitionMs === 'number' ? extra.closePageTransitionMs : 220),
  closeButtonAnimation:
    process.env.EXPO_PUBLIC_CLOSE_BUTTON_ANIMATION ??
    (typeof extra.closeButtonAnimation === 'string'
      ? extra.closeButtonAnimation
      : 'subtle')
};
