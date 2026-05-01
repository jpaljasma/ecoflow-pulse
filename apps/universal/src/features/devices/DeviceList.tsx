import { FlatList, Platform, View } from 'react-native';
import { YStack } from 'tamagui';
import { forwardRef, useImperativeHandle, useRef, type ReactElement } from 'react';
import type { DeviceSummary } from '@/features/devices/api';
import type { TelemetryEngineStatus } from '@/features/telemetry/engine/types';
import { DeviceCard } from '@/features/devices/DeviceCard';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';

export type DeviceListHandle = {
  scrollToDevices: () => void;
};

export const DeviceList = forwardRef<DeviceListHandle, {
  devices: DeviceSummary[];
  connectionStatus: TelemetryEngineStatus;
  header?: ReactElement;
  footer?: ReactElement;
}>(function DeviceList({
  devices,
  connectionStatus,
  header,
  footer
}: {
  devices: DeviceSummary[];
  connectionStatus: TelemetryEngineStatus;
  header?: ReactElement;
  footer?: ReactElement;
}, ref) {
  const { contentWidth } = useNavigationShellMetrics();
  const columns = Platform.OS === 'web' && contentWidth >= 900 ? 2 : 1;
  const listRef = useRef<FlatList<DeviceSummary>>(null);

  useImperativeHandle(
    ref,
    () => ({
      scrollToDevices: () => {
        if (!devices.length) return;
        listRef.current?.scrollToIndex({
          index: 0,
          animated: true,
          viewPosition: 0
        });
      }
    }),
    [devices.length]
  );

  return (
    <FlatList
      ref={listRef}
      key={`device-list-${columns}`}
      data={devices}
      keyExtractor={(item) => item.id}
      numColumns={columns}
      contentContainerStyle={{ padding: 16, paddingBottom: 16 }}
      ListHeaderComponent={header}
      ListFooterComponent={footer}
      columnWrapperStyle={columns > 1 ? { gap: 12 } : undefined}
      removeClippedSubviews
      initialNumToRender={8}
      maxToRenderPerBatch={10}
      windowSize={7}
      updateCellsBatchingPeriod={50}
      onScrollToIndexFailed={() => {
        if (!devices.length) return;
        requestAnimationFrame(() => {
          listRef.current?.scrollToIndex({
            index: 0,
            animated: true,
            viewPosition: 0
          });
        });
      }}
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
});
