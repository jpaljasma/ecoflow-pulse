import { Platform, FlatList, useWindowDimensions } from 'react-native';
import { YStack } from 'tamagui';
import type { DeviceSummary } from '@/features/devices/api';
import type { DeviceSnapshot } from '@/features/telemetry/engine/types';
import { DeviceCard } from '@/features/devices/DeviceCard';

export function DeviceList({
  devices,
  byId
}: {
  devices: DeviceSummary[];
  byId: Record<string, DeviceSnapshot>;
}) {
  const { width } = useWindowDimensions();

  if (Platform.OS === 'web') {
    const columns = width >= 900 ? 2 : 1;
    return (
      <YStack paddingHorizontal="$4" paddingBottom="$4">
        <div
          style={{
            display: 'grid',
            gridTemplateColumns: `repeat(${columns}, minmax(0, 1fr))`,
            gap: 12
          }}
        >
          {devices.map((device) => (
            <DeviceCard
              key={device.id}
              device={device}
              snapshot={byId[device.id]}
              imageContext="list"
            />
          ))}
        </div>
      </YStack>
    );
  }

  return (
    <FlatList
      data={devices}
      keyExtractor={(item) => item.id}
      contentContainerStyle={{ padding: 16, gap: 12 }}
      removeClippedSubviews
      initialNumToRender={8}
      maxToRenderPerBatch={10}
      windowSize={7}
      renderItem={({ item }) => (
        <DeviceCard device={item} snapshot={byId[item.id]} imageContext="list" />
      )}
    />
  );
}
