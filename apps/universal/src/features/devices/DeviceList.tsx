import { Platform, FlatList } from 'react-native';
import { FixedSizeList as WindowList } from 'react-window';
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
  if (Platform.OS === 'web') {
    const height = Math.min(900, Math.max(520, window.innerHeight - 180));
    return (
    <YStack paddingHorizontal="$4" paddingBottom="$4">
        <WindowList height={height} itemCount={devices.length} itemSize={190} width="100%">
          {({ index, style }) => {
            const device = devices[index] as DeviceSummary;
            return (
              <div style={{ ...style, paddingBottom: 12 }}>
                <DeviceCard device={device} snapshot={byId[device.id]} />
              </div>
            );
          }}
        </WindowList>
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
      renderItem={({ item }) => <DeviceCard device={item} snapshot={byId[item.id]} />}
    />
  );
}
