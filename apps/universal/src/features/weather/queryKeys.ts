export function buildWeatherQueryKey(authKey: string, locationKey: string): readonly [string, string, string] {
  return ['weather', authKey, locationKey] as const;
}
