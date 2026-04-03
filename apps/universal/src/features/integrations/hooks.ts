import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  createIntegration,
  fetchIntegrations,
  setIntegrationActive,
  updateIntegration
} from '@/features/integrations/api';
import type {
  CreateIntegrationPayload,
  SetIntegrationActivePayload,
  UpdateIntegrationPayload
} from '@/features/integrations/schema';

type IntegrationQueryOptions = {
  token?: string;
  authKey?: string;
  enabled?: boolean;
  provider?: string;
};

function invalidateIntegrationRelatedQueries(queryClient: ReturnType<typeof useQueryClient>, authKey: string) {
  void queryClient.invalidateQueries({ queryKey: ['integrations', authKey] });
  void queryClient.invalidateQueries({ queryKey: ['available-devices', authKey] });
  void queryClient.invalidateQueries({ queryKey: ['devices', authKey] });
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
    onSuccess: () => invalidateIntegrationRelatedQueries(queryClient, authKey)
  });
}

export function useUpdateIntegration(options: IntegrationQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: { credentialId: string; values: UpdateIntegrationPayload }) =>
      updateIntegration(payload.credentialId, payload.values, token),
    onSuccess: () => invalidateIntegrationRelatedQueries(queryClient, authKey)
  });
}

export function useSetIntegrationActive(options: IntegrationQueryOptions = {}) {
  const { token, authKey = 'anonymous' } = options;
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (payload: { credentialId: string; values: SetIntegrationActivePayload }) =>
      setIntegrationActive(payload.credentialId, payload.values, token),
    onSuccess: () => invalidateIntegrationRelatedQueries(queryClient, authKey)
  });
}
