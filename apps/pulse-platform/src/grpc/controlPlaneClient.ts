import path from 'node:path';
import { fileURLToPath } from 'node:url';

import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';

export type UserDevice = {
  deviceId: string;
  ecoflowSn: string;
  productName: string;
  model: string;
  role: string;
  createdAtUnixMs: string;
  updatedAtUnixMs: string;
};

export type ProviderDevice = {
  id: string;
  deviceId: string;
  provider: string;
  providerDeviceId: string;
  credentialId: string;
  canonicalSn: string;
  productName: string;
  model: string;
  isActive: boolean;
  ingestDesiredState: string;
  capabilities?: Record<string, unknown>;
  metadata?: Record<string, unknown>;
};

export type ProviderDeviceGroup = {
  provider: string;
  devices: ProviderDevice[];
};

export type ListUserDevicesInput = {
  userSubject: string;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export type ListDevicesInput = {
  userSubject: string;
  provider?: string;
  activeOnly?: boolean;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export interface ControlPlaneClient {
  listUserDevices(input: ListUserDevicesInput): Promise<UserDevice[]>;
  listDevices(input: ListDevicesInput): Promise<ProviderDeviceGroup[]>;
  close(): void;
}

type GrpcUnaryMethod = (
  request: Record<string, unknown>,
  metadata: grpc.Metadata,
  options: grpc.CallOptions,
  callback: (error: grpc.ServiceError | null, response?: unknown) => void
) => void;

type GrpcControlPlaneClient = {
  ListUserDevices: GrpcUnaryMethod;
  ListDevices: GrpcUnaryMethod;
  close: () => void;
};

type ControlPlaneProto = {
  pulse: {
    controlplane: {
      v1: {
        ControlPlaneService: new (
          address: string,
          credentials: grpc.ChannelCredentials,
          options?: Record<string, unknown>
        ) => GrpcControlPlaneClient;
      };
    };
  };
};

type RawUserDevice = Partial<Record<keyof UserDevice, unknown>>;
type RawListUserDevicesResponse = {
  devices?: unknown;
};

type RawProviderDevice = Partial<Record<keyof ProviderDevice, unknown>>;
type RawProviderDeviceGroup = {
  provider?: unknown;
  devices?: unknown;
};
type RawListDevicesResponse = {
  groups?: unknown;
};

const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);
const projectRoot = path.resolve(__dirname, '../../../../');
const protoRoot = path.join(projectRoot, 'proto');
const controlPlaneProtoPath = path.join(protoRoot, 'pulse/controlplane/v1/control_plane.proto');

const packageDefinition = protoLoader.loadSync(controlPlaneProtoPath, {
  keepCase: false,
  longs: String,
  enums: String,
  defaults: true,
  oneofs: true,
  includeDirs: [protoRoot]
});
const controlPlaneProto = grpc.loadPackageDefinition(packageDefinition) as unknown as ControlPlaneProto;

export function createControlPlaneClient(address: string): ControlPlaneClient {
  const client = new controlPlaneProto.pulse.controlplane.v1.ControlPlaneService(
    address,
    grpc.credentials.createInsecure()
  );
  return {
    async listUserDevices(input) {
      const response = await unaryCall<RawListUserDevicesResponse>(
        client.ListUserDevices.bind(client),
        { userSubject: input.userSubject },
        input
      );
      if (!Array.isArray(response.devices)) {
        return [];
      }
      return response.devices.map((row) => normalizeUserDevice(row as RawUserDevice));
    },
    async listDevices(input) {
      const response = await unaryCall<RawListDevicesResponse>(
        client.ListDevices.bind(client),
        {
          userSubject: input.userSubject,
          provider: input.provider ?? '',
          activeOnly: input.activeOnly ?? false
        },
        input
      );
      if (!Array.isArray(response.groups)) {
        return [];
      }
      return response.groups.map((row) => normalizeProviderDeviceGroup(row as RawProviderDeviceGroup));
    },
    close() {
      client.close();
    }
  };
}

function unaryCall<T>(
  method: GrpcUnaryMethod,
  request: Record<string, unknown>,
  input: { authHeader?: string; requestID?: string; deadlineMs: number }
): Promise<T> {
  const metadata = new grpc.Metadata();
  if (input.authHeader) {
    metadata.set('authorization', input.authHeader);
  }
  if (input.requestID) {
    metadata.set('x-request-id', input.requestID);
  }
  return new Promise<T>((resolve, reject) => {
    method(
      request,
      metadata,
      { deadline: new Date(Date.now() + input.deadlineMs) },
      (error, response) => {
        if (error) {
          reject(error);
          return;
        }
        resolve(response as T);
      }
    );
  });
}

function normalizeUserDevice(device: RawUserDevice): UserDevice {
  return {
    deviceId: normalizeString(device.deviceId),
    ecoflowSn: normalizeString(device.ecoflowSn),
    productName: normalizeString(device.productName),
    model: normalizeString(device.model),
    role: normalizeString(device.role),
    createdAtUnixMs: normalizeString(device.createdAtUnixMs),
    updatedAtUnixMs: normalizeString(device.updatedAtUnixMs)
  };
}

function normalizeProviderDeviceGroup(group: RawProviderDeviceGroup): ProviderDeviceGroup {
  return {
    provider: normalizeString(group.provider),
    devices: Array.isArray(group.devices)
      ? group.devices.map((row) => normalizeProviderDevice(row as RawProviderDevice))
      : []
  };
}

function normalizeProviderDevice(device: RawProviderDevice): ProviderDevice {
  return {
    id: normalizeString(device.id),
    deviceId: normalizeString(device.deviceId),
    provider: normalizeString(device.provider),
    providerDeviceId: normalizeString(device.providerDeviceId),
    credentialId: normalizeString(device.credentialId),
    canonicalSn: normalizeString(device.canonicalSn),
    productName: normalizeString(device.productName),
    model: normalizeString(device.model),
    isActive: Boolean(device.isActive),
    ingestDesiredState: normalizeString(device.ingestDesiredState),
    capabilities: normalizeRecord(device.capabilities),
    metadata: normalizeRecord(device.metadata)
  };
}

function normalizeString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function normalizeRecord(value: unknown): Record<string, unknown> | undefined {
  const normalized = normalizeProtoValue(value);
  if (!normalized || typeof normalized !== 'object' || Array.isArray(normalized)) {
    return undefined;
  }
  return normalized as Record<string, unknown>;
}

function normalizeProtoValue(value: unknown): unknown {
  if (value === null || value === undefined) {
    return undefined;
  }
  if (Array.isArray(value)) {
    return value.map((item) => normalizeProtoValue(item));
  }
  if (typeof value !== 'object') {
    return value;
  }

  const record = value as Record<string, unknown>;

  if ('fields' in record && typeof record.fields === 'object' && record.fields !== null && !Array.isArray(record.fields)) {
    const out: Record<string, unknown> = {};
    for (const [key, fieldValue] of Object.entries(record.fields as Record<string, unknown>)) {
      out[key] = normalizeProtoValue(fieldValue);
    }
    return out;
  }

  if ('kind' in record && typeof record.kind === 'string') {
    const kind = record.kind;
    const kindValue = record[kind];
    return normalizeProtoValue(kindValue);
  }

  const out: Record<string, unknown> = {};
  for (const [key, nested] of Object.entries(record)) {
    out[key] = normalizeProtoValue(nested);
  }
  return out;
}
