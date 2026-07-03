import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  connectEcoFlowBLEAuth,
  createIntegration,
  fetchEcoFlowBLEAuthStatus,
  fetchIntegrations,
  setEcoFlowBLEAuthUserID,
  setIntegrationActive,
  updateIntegration
} from '@/features/integrations/api';
import type {
  ConnectEcoFlowBLEAuthPayload,
  CreateIntegrationPayload,
  SetEcoFlowBLEAuthUserIDPayload,
  SetIntegrationActivePayload,
  UpdateIntegrationPayload
} from '@/features/integrations/schema';

type IntegrationQueryOptions = {
  token?: string;
  authKey?: string;
  enabled?: boolean;
  provider?: string;
};

function invalidateProviderIntegrationQueries(queryClient: ReturnType<typeof useQueryClient>, authKey: string) {
  void queryClient.invalidateQueries({ queryKey: ['integrations', authKey] });
  void queryClient.invalidateQueries({ queryKey: ['available-devices', authKey] });
  void queryClient.invalidateQueries({ queryKey: ['devices', authKey] });
}

function invalidateBLEAuthQueries(queryClient: ReturnType<typeof useQueryClient>, authKey: string) {
  void queryClient.invalidateQueries({ queryKey: ['ecoflow-ble-auth', authKey] });
  void queryClient.invalidateQueries({ queryKey: ['edge-collectors', authKey] });
  void queryClient.invalidateQueries({ queryKey: ['edge-device-sources', authKey] });
}

export function useIntegrations(options: IntegrationQueryOptions = {}) {
  const { token, authKey = 'anonymous', enabled = true, provider } = options;
  return useQuery({
    queryKey: ['integrations', authKey, provider ?? 'all'],
    queryFn: () => fetchIntegrations(token, provider),
    enabled,
    staleTime: 0,
    gcTime: 60_000,
    placeholderData: (previous) => previous
  });
}

export function useCreateIntegration(options: IntegrationQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: CreateIntegrationPayload) => createIntegration(payload, token),
    onSuccess: () => invalidateProviderIntegrationQueries(queryClient, authKey)
  });
}

export function useUpdateIntegration(options: IntegrationQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: { credentialId: string; values: UpdateIntegrationPayload }) =>
      updateIntegration(payload.credentialId, payload.values, token),
    onSuccess: () => invalidateProviderIntegrationQueries(queryClient, authKey)
  });
}

export function useSetIntegrationActive(options: IntegrationQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: { credentialId: string; values: SetIntegrationActivePayload }) =>
      setIntegrationActive(payload.credentialId, payload.values, token),
    onSuccess: () => invalidateProviderIntegrationQueries(queryClient, authKey)
  });
}

export function useEcoFlowBLEAuthStatus(options: IntegrationQueryOptions = {}) {
  const { token, authKey = 'anonymous', enabled = true } = options;
  return useQuery({
    queryKey: ['ecoflow-ble-auth', authKey],
    queryFn: () => fetchEcoFlowBLEAuthStatus(token),
    enabled,
    staleTime: 30_000,
    gcTime: 5 * 60_000
  });
}

export function useConnectEcoFlowBLEAuth(options: IntegrationQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: ConnectEcoFlowBLEAuthPayload) => connectEcoFlowBLEAuth(payload, token),
    onSuccess: () => invalidateBLEAuthQueries(queryClient, authKey)
  });
}

export function useSetEcoFlowBLEAuthUserID(options: IntegrationQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: SetEcoFlowBLEAuthUserIDPayload) => setEcoFlowBLEAuthUserID(payload, token),
    onSuccess: () => invalidateBLEAuthQueries(queryClient, authKey)
  });
}
