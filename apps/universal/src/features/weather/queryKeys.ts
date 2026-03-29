export function buildWeatherQueryKey(
  authKey: string,
  locationKey: string,
  scope: 'all' | 'device' = 'all',
  deviceId = ''
): readonly [string, string, string, string, string] {
  return ['weather', authKey, locationKey, scope, deviceId] as const;
}
