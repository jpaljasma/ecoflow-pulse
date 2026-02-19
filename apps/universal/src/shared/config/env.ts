import Constants from 'expo-constants';
import { Platform } from 'react-native';

const extra = Constants.expoConfig?.extra ?? {};
const defaultWebAssetBaseUrl =
  Platform.OS === 'web' && typeof window !== 'undefined' && window.location?.origin
    ? window.location.origin
    : '';

export const env = {
  defaultAssetBaseUrl: defaultWebAssetBaseUrl,
  apiUrl:
    process.env.EXPO_PUBLIC_API_URL ??
    (typeof extra.apiUrl === 'string' ? extra.apiUrl : 'mock://ecoflow'),
  wsUrl:
    process.env.EXPO_PUBLIC_WS_URL ??
    (typeof extra.wsUrl === 'string' ? extra.wsUrl : 'ws://localhost:8080/ws'),
  mockLogUrl:
    process.env.EXPO_PUBLIC_MOCK_LOG_URL ??
    (typeof extra.mockLogUrl === 'string' ? extra.mockLogUrl : '/mock/mqtt.log'),
  mockTrainingUrl:
    process.env.EXPO_PUBLIC_MOCK_TRAINING_URL ??
    (typeof extra.mockTrainingUrl === 'string'
      ? extra.mockTrainingUrl
      : '/mock/telemetry_training.csv'),
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
