import { FlatList, Platform, View, useWindowDimensions } from 'react-native';
import { YStack } from 'tamagui';
import type { ReactElement } from 'react';
import type { DeviceSummary } from '@/features/devices/api';
import type { TelemetryEngineStatus } from '@/features/telemetry/engine/types';
import { DeviceCard } from '@/features/devices/DeviceCard';

export function DeviceList({
  devices,
  connectionStatus,
  header
}: {
  devices: DeviceSummary[];
  connectionStatus: TelemetryEngineStatus;
  header?: ReactElement;
}) {
  const { width } = useWindowDimensions();
  const columns = Platform.OS === 'web' && width >= 900 ? 2 : 1;

  return (
    <FlatList
      key={`device-list-${columns}`}
      data={devices}
      keyExtractor={(item) => item.id}
      numColumns={columns}
      contentContainerStyle={{ padding: 16, paddingBottom: 16 }}
      ListHeaderComponent={header}
      columnWrapperStyle={columns > 1 ? { gap: 12 } : undefined}
      removeClippedSubviews
      initialNumToRender={8}
      maxToRenderPerBatch={10}
      windowSize={7}
      updateCellsBatchingPeriod={50}
      ItemSeparatorComponent={() => <YStack height="$3" />}
      renderItem={({ item }) => (
        <View style={{ flex: 1, minWidth: 0 }}>
          <DeviceCard
            device={item}
            imageContext="list"
            connectionStatus={connectionStatus}
          />
        </View>
      )}
    />
  );
}
