import Constants from 'expo-constants';

const extra = Constants.expoConfig?.extra ?? {};

export const env = {
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
