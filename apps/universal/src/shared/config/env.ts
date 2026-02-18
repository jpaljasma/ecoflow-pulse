import Constants from 'expo-constants';

const extra = Constants.expoConfig?.extra ?? {};

export const env = {
  apiUrl:
    process.env.EXPO_PUBLIC_API_URL ??
    (typeof extra.apiUrl === 'string' ? extra.apiUrl : 'mock://ecoflow'),
  wsUrl:
    process.env.EXPO_PUBLIC_WS_URL ??
    (typeof extra.wsUrl === 'string' ? extra.wsUrl : 'ws://localhost:8080/ws')
};
