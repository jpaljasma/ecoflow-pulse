import {
  FlatList,
  Platform,
  View
} from 'react-native';
import { YStack } from 'tamagui';
import {
  forwardRef,
  useCallback,
  useImperativeHandle,
  useRef,
  useState,
  type ReactElement
} from 'react';
import type { DeviceSummary } from '@/features/devices/api';
import type { TelemetryEngineStatus } from '@/features/telemetry/engine/types';
import { DeviceCard } from '@/features/devices/DeviceCard';
import { findDeviceScrollIndex } from '@/features/devices/listScroll';
import { useNavigationShellMetrics } from '@/shared/ui/navigationShell';

export type DeviceListHandle = {
  scrollToDevices: () => void;
  scrollToDevice: (deviceId: string) => void;
};

export const DeviceList = forwardRef<DeviceListHandle, {
  devices: DeviceSummary[];
  connectionStatus: TelemetryEngineStatus;
  highlightedDeviceId?: string;
  header?: ReactElement;
  footer?: ReactElement;
}>(function DeviceList({
  devices,
  connectionStatus,
  highlightedDeviceId,
  header,
  footer
}: {
  devices: DeviceSummary[];
  connectionStatus: TelemetryEngineStatus;
  highlightedDeviceId?: string;
  header?: ReactElement;
  footer?: ReactElement;
}, ref) {
  const { contentWidth } = useNavigationShellMetrics();
  const columns = Platform.OS === 'web' && contentWidth >= 900 ? 2 : 1;
  const listRef = useRef<FlatList<DeviceSummary>>(null);
  const [solarHistoryDeviceIds, setSolarHistoryDeviceIds] = useState<Set<string>>(() => new Set());
  const viewabilityConfig = useRef({
    itemVisiblePercentThreshold: 12,
    minimumViewTime: 100
  }).current;
  const onViewableItemsChanged = useRef(
    ({ viewableItems }: { viewableItems: Array<{ item?: DeviceSummary; isViewable?: boolean }> }) => {
      const nextVisibleIds = viewableItems
        .filter((item) => item.isViewable && item.item?.id)
        .map((item) => item.item?.id)
        .filter((id): id is string => Boolean(id));
      if (!nextVisibleIds.length) return;
      setSolarHistoryDeviceIds((previous) => {
        let changed = false;
        const next = new Set(previous);
        for (const id of nextVisibleIds) {
          if (!next.has(id)) {
            next.add(id);
            changed = true;
          }
        }
        return changed ? next : previous;
      });
    }
  ).current;
  const renderDeviceCard = useCallback(
    ({ item }: { item: DeviceSummary }) => (
      <View style={{ flex: 1, minWidth: 0 }}>
        <DeviceCard
          device={item}
          imageContext="list"
          connectionStatus={connectionStatus}
          highlighted={item.id === highlightedDeviceId}
          loadSolarHistory={solarHistoryDeviceIds.has(item.id)}
        />
      </View>
    ),
    [connectionStatus, highlightedDeviceId, solarHistoryDeviceIds]
  );

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
      },
      scrollToDevice: (deviceId: string) => {
        const index = findDeviceScrollIndex(devices, deviceId);
        if (index < 0) return;
        listRef.current?.scrollToIndex({
          index,
          animated: true,
          viewPosition: 0.08
        });
      }
    }),
    [devices]
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
      viewabilityConfig={viewabilityConfig}
      onViewableItemsChanged={onViewableItemsChanged}
      onScrollToIndexFailed={({ index }) => {
        if (!devices.length) return;
        requestAnimationFrame(() => {
          listRef.current?.scrollToIndex({
            index: Math.max(0, Math.min(index, devices.length - 1)),
            animated: true,
            viewPosition: 0.08
          });
        });
      }}
      ItemSeparatorComponent={() => <YStack height="$3" />}
      renderItem={renderDeviceCard}
    />
  );
});
