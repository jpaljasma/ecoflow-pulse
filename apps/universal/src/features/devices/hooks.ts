import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  approveEdgeDeviceSource,
  createEdgeCollector,
  EDGE_DEVICE_SOURCE_STATUS_PENDING,
  enableAvailableDevice,
  fetchAvailableDevices,
  fetchDevice,
  fetchDevices,
  fetchEdgeCollectors,
  fetchEdgeDeviceSources,
  importAvailableDevice,
  revokeEdgeCollectorSetupToken,
  testAvailableDeviceMQTT
} from '@/features/devices/api';
import type {
  ApproveEdgeDeviceSourcePayload,
  CreateEdgeCollectorPayload,
  EdgeDeviceSourceFilterStatus,
  ImportAvailableDevicePayload
} from '@/features/devices/schema';

type DeviceQueryOptions = {
  token?: string;
  authKey?: string;
  enabled?: boolean;
};

export function useDevices(options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  return useQuery({
    queryKey: ['devices', authKey],
    queryFn: () => fetchDevices(token),
    enabled,
    staleTime: 60_000,
    gcTime: 5 * 60_000,
    placeholderData: (previous) => previous
  });
}

export function useDevice(
  deviceId: string | undefined,
  options: DeviceQueryOptions = {}
) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  return useQuery({
    queryKey: ['device', deviceId, authKey],
    queryFn: () => fetchDevice(deviceId ?? '', token),
    enabled: enabled && Boolean(deviceId),
    staleTime: 60_000,
    gcTime: 5 * 60_000,
    placeholderData: (previous) => previous
  });
}

export function useAvailableDevices(options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  return useQuery({
    queryKey: ['available-devices', authKey],
    queryFn: () => fetchAvailableDevices(token),
    enabled,
    staleTime: 0,
    gcTime: 0,
    refetchOnMount: 'always'
  });
}

export function useTestAvailableDeviceMQTT(options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: {
      provider: string;
      credentialId: string;
      providerDeviceId: string;
    }) => testAvailableDeviceMQTT(payload, token),
    onSuccess: (result) => {
      if (!result.success) {
        return;
      }
      void queryClient.invalidateQueries({ queryKey: ['devices', authKey] });
      void queryClient.invalidateQueries({
        queryKey: ['available-devices', authKey]
      });
    }
  });
}

export function useEnableAvailableDevice(options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: {
      provider: string;
      credentialId: string;
      providerDeviceId: string;
    }) => enableAvailableDevice(payload, token),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['devices', authKey] });
      void queryClient.invalidateQueries({
        queryKey: ['available-devices', authKey]
      });
    }
  });
}

export function useImportAvailableDevice(options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: ImportAvailableDevicePayload) =>
      importAvailableDevice(payload, token),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['devices', authKey] });
      void queryClient.invalidateQueries({
        queryKey: ['available-devices', authKey]
      });
    }
  });
}

export function useEdgeCollectors(options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  return useQuery({
    queryKey: ['edge-collectors', authKey],
    queryFn: () => fetchEdgeCollectors(token),
    enabled,
    staleTime: 30_000,
    gcTime: 5 * 60_000
  });
}

export function useCreateEdgeCollector(options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateEdgeCollectorPayload) =>
      createEdgeCollector(payload, token),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['edge-collectors', authKey]
      });
    }
  });
}

export function useRevokeEdgeCollectorSetupToken(
  options: DeviceQueryOptions = {}
) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (collectorId: string) =>
      revokeEdgeCollectorSetupToken(collectorId, token),
    onSuccess: () => {
      void queryClient.invalidateQueries({
        queryKey: ['edge-collectors', authKey]
      });
    }
  });
}

export function useEdgeDeviceSources(
  options: DeviceQueryOptions & {
    status?: EdgeDeviceSourceFilterStatus;
    includeAllStatuses?: boolean;
  } = {}
) {
  const {
    token,
    authKey = 'anonymous',
    enabled = true,
    includeAllStatuses = false,
    status = EDGE_DEVICE_SOURCE_STATUS_PENDING
  } = options;
  const queryStatus = includeAllStatuses ? undefined : status;
  return useQuery({
    queryKey: ['edge-device-sources', authKey, queryStatus ?? 'all'],
    queryFn: () => fetchEdgeDeviceSources(token, queryStatus),
    enabled,
    staleTime: 10_000,
    gcTime: 60_000,
    refetchOnMount: 'always'
  });
}

export function useApproveEdgeDeviceSource(options: DeviceQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: ApproveEdgeDeviceSourcePayload) =>
      approveEdgeDeviceSource(payload, token),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['devices', authKey] });
      void queryClient.invalidateQueries({
        queryKey: ['available-devices', authKey]
      });
      void queryClient.invalidateQueries({
        queryKey: ['edge-device-sources', authKey]
      });
    }
  });
}
