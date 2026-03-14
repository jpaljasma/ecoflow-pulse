import * as Location from 'expo-location';

export type DetectedWeatherLocation = {
  label: string;
  latitude: number;
  longitude: number;
};

export async function detectCurrentWeatherLocation(): Promise<DetectedWeatherLocation> {
  const permission = await Location.requestForegroundPermissionsAsync();
  if (!permission.granted) {
    throw new Error('Location permission not granted');
  }
  const position = await Location.getCurrentPositionAsync({
    accuracy: Location.Accuracy.Balanced
  });
  const latitude = position.coords.latitude;
  const longitude = position.coords.longitude;
  const label = await reverseGeocodeLabel(latitude, longitude);
  return {
    label,
    latitude,
    longitude
  };
}

async function reverseGeocodeLabel(latitude: number, longitude: number): Promise<string> {
  try {
    const rows = await Location.reverseGeocodeAsync({ latitude, longitude });
    const first = rows[0];
    if (!first) {
      return `${latitude.toFixed(3)}, ${longitude.toFixed(3)}`;
    }
    const parts = [first.city, first.region, first.country]
      .filter((value): value is string => typeof value === 'string' && value.trim().length > 0);
    return parts.length > 0 ? parts.join(', ') : `${latitude.toFixed(3)}, ${longitude.toFixed(3)}`;
  } catch {
    return `${latitude.toFixed(3)}, ${longitude.toFixed(3)}`;
  }
}
