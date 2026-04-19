import { requestJson } from '@/shared/api/restClient';
import {
  AvailableDevicesResponseSchema,
  DeviceMQTTTestResultSchema,
  DevicesResponseSchema,
  DeviceSchema,
  EnableAvailableDeviceResponseSchema,
  ImportAvailableDevicePayloadSchema
} from '@/features/devices/schema';
import type {
  AvailableDevicesResponse,
  DeviceMQTTTestResult,
  DevicesResponse,
  DeviceSummary,
  EnableAvailableDeviceResponse,
  ImportAvailableDevicePayload
} from '@/features/devices/schema';

export {
  AvailableDeviceSchema,
  AvailableDevicesResponseSchema,
  DeviceMQTTTestResultSchema,
  DevicesResponseSchema,
  DeviceSchema,
  EnableAvailableDeviceResponseSchema
} from '@/features/devices/schema';

export type {
  AvailableDeviceSummary,
  AvailableDevicesResponse,
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
