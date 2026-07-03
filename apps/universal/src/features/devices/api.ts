import { requestJson } from '@/shared/api/restClient';
import {
  AvailableDevicesResponseSchema,
  ApproveEdgeDeviceSourcePayloadSchema,
  ApproveEdgeDeviceSourceResponseSchema,
  CreateEdgeCollectorPayloadSchema,
  CreateEdgeCollectorResponseSchema,
  DeviceMQTTTestResultSchema,
  DevicesResponseSchema,
  DeviceSchema,
  EdgeCollectorsResponseSchema,
  EdgeDeviceSourcesResponseSchema,
  EdgeDeviceSourceStatusSchema,
  EnableAvailableDeviceResponseSchema,
  ImportAvailableDevicePayloadSchema
} from '@/features/devices/schema';
import type {
  ApproveEdgeDeviceSourcePayload,
  ApproveEdgeDeviceSourceResponse,
  AvailableDevicesResponse,
  DeviceMQTTTestResult,
  DevicesResponse,
  DeviceSummary,
  CreateEdgeCollectorPayload,
  CreateEdgeCollectorResponse,
  EdgeCollector,
  EdgeDeviceSourceStatus,
  EdgeDeviceSource,
  EnableAvailableDeviceResponse,
  ImportAvailableDevicePayload
} from '@/features/devices/schema';

export {
  AvailableDeviceSchema,
  AvailableDevicesResponseSchema,
  EdgeCollectorSchema,
  EdgeDeviceSourceStatusSchema,
  EdgeDeviceSourceSchema,
  DeviceMQTTTestResultSchema,
  DevicesResponseSchema,
  DeviceSchema,
  EnableAvailableDeviceResponseSchema
} from '@/features/devices/schema';

export type {
  AvailableDeviceSummary,
  AvailableDevicesResponse,
  EdgeCollector,
  EdgeDeviceSourceStatus,
  EdgeDeviceSource,
  DeviceMQTTTestResult,
  DevicesResponse,
  DeviceSummary,
  EnableAvailableDeviceResponse
} from '@/features/devices/schema';

export async function fetchDevices(token?: string): Promise<DevicesResponse> {
  const data = await requestJson<unknown>('/api/devices', { token });
  return DevicesResponseSchema.parse(data);
}

export async function fetchDevice(
  deviceId: string,
  token?: string
): Promise<DeviceSummary> {
  const data = await requestJson<unknown>(`/api/devices/${deviceId}`, { token });
  return DeviceSchema.parse(data);
}

export async function fetchAvailableDevices(token?: string): Promise<AvailableDevicesResponse> {
  const data = await requestJson<unknown>('/api/v1/devices/available', { token });
  return AvailableDevicesResponseSchema.parse(data);
}

export async function testAvailableDeviceMQTT(
  payload: { provider: string; credentialId: string; providerDeviceId: string },
  token?: string
): Promise<DeviceMQTTTestResult> {
  const data = await requestJson<unknown>('/api/v1/devices/available/test-mqtt', {
    method: 'POST',
    body: payload,
    token
  });
  return DeviceMQTTTestResultSchema.parse(data);
}

export async function enableAvailableDevice(
  payload: { provider: string; credentialId: string; providerDeviceId: string },
  token?: string
): Promise<EnableAvailableDeviceResponse> {
  const data = await requestJson<unknown>('/api/v1/devices/available/enable', {
    method: 'POST',
    body: payload,
    token
  });
  return EnableAvailableDeviceResponseSchema.parse(data);
}

export async function importAvailableDevice(
  payload: ImportAvailableDevicePayload,
  token?: string
): Promise<EnableAvailableDeviceResponse> {
  const validated = ImportAvailableDevicePayloadSchema.parse(payload);
  const data = await requestJson<unknown>('/api/v1/devices/available/import', {
    method: 'POST',
    body: validated,
    token
  });
  return EnableAvailableDeviceResponseSchema.parse(data);
}

export async function fetchEdgeCollectors(token?: string): Promise<EdgeCollector[]> {
  const data = await requestJson<unknown>('/api/v1/edge/collectors', { token });
  return EdgeCollectorsResponseSchema.parse(data).collectors;
}

export async function createEdgeCollector(
  payload: CreateEdgeCollectorPayload,
  token?: string
): Promise<CreateEdgeCollectorResponse> {
  const validated = CreateEdgeCollectorPayloadSchema.parse(payload);
  const data = await requestJson<unknown>('/api/v1/edge/collectors', {
    method: 'POST',
    body: validated,
    token
  });
  return CreateEdgeCollectorResponseSchema.parse(data);
}

export async function fetchEdgeDeviceSources(
  token?: string,
  status: EdgeDeviceSourceStatus = 'pending'
): Promise<EdgeDeviceSource[]> {
  const validatedStatus = EdgeDeviceSourceStatusSchema.parse(status);
  const suffix = `?status=${encodeURIComponent(validatedStatus)}`;
  const data = await requestJson<unknown>(`/api/v1/edge/device-sources${suffix}`, { token });
  return EdgeDeviceSourcesResponseSchema.parse(data).sources;
}

export async function approveEdgeDeviceSource(
  payload: ApproveEdgeDeviceSourcePayload,
  token?: string
): Promise<ApproveEdgeDeviceSourceResponse> {
  const validated = ApproveEdgeDeviceSourcePayloadSchema.parse(payload);
  const data = await requestJson<unknown>(`/api/v1/edge/device-sources/${encodeURIComponent(validated.sourceId)}/approve`, {
    method: 'POST',
    body: {
      deviceId: validated.deviceId,
      productName: validated.productName,
      model: validated.model
    },
    token
  });
  return ApproveEdgeDeviceSourceResponseSchema.parse(data);
}
