import { requestJson } from '@/shared/api/restClient';
import {
  CreateIntegrationPayloadSchema,
  type CreateIntegrationPayload,
  type Integration,
  type IntegrationsResponse,
  IntegrationsResponseSchema,
  IntegrationResponseSchema,
  SetIntegrationActivePayloadSchema,
  type SetIntegrationActivePayload,
  UpdateIntegrationPayloadSchema,
  type UpdateIntegrationPayload
} from '@/features/integrations/schema';

export async function fetchIntegrations(
  token?: string,
  provider?: string
): Promise<IntegrationsResponse> {
  const suffix = provider ? `?provider=${encodeURIComponent(provider)}` : '';
  const data = await requestJson<unknown>(`/api/v1/integrations${suffix}`, { token });
  return IntegrationsResponseSchema.parse(data);
}

export async function createIntegration(
  payload: CreateIntegrationPayload,
  token?: string
): Promise<Integration> {
  const validated = CreateIntegrationPayloadSchema.parse(payload);
  const data = await requestJson<unknown>('/api/v1/integrations', {
    method: 'POST',
    token,
    body: validated
  });
  return IntegrationResponseSchema.parse(data).integration;
}

export async function updateIntegration(
  credentialId: string,
  payload: UpdateIntegrationPayload,
  token?: string
): Promise<Integration> {
  const validated = UpdateIntegrationPayloadSchema.parse(payload);
  const data = await requestJson<unknown>(`/api/v1/integrations/${credentialId}`, {
    method: 'PATCH',
    token,
    body: validated
  });
  return IntegrationResponseSchema.parse(data).integration;
}

export async function setIntegrationActive(
  credentialId: string,
  payload: SetIntegrationActivePayload,
  token?: string
): Promise<Integration> {
  const validated = SetIntegrationActivePayloadSchema.parse(payload);
  const data = await requestJson<unknown>(`/api/v1/integrations/${credentialId}/active`, {
    method: 'PATCH',
    token,
    body: validated
  });
  return IntegrationResponseSchema.parse(data).integration;
}
