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

export type CurrentUser = {
  id: string;
  keycloakSubject: string;
  email: string;
  emailVerified: boolean;
  displayName: string;
  displayNameSource: string;
  avatarUrl: string;
  givenName: string;
  familyName: string;
  locale: string;
  timezone: string;
  weatherLocationEnabled: boolean;
  weatherLocationSource: string;
  weatherLocationLabel: string;
  weatherLatitude?: number;
  weatherLongitude?: number;
  hasWeatherLocation: boolean;
  lastLoginAtUnixMs: string;
  createdAtUnixMs: string;
  updatedAtUnixMs: string;
  authMethod: string;
};

export type AuthorizationSummary = {
  tokenRoles: string[];
  deviceCount: number;
};

export type CurrentUserBootstrap = {
  user: CurrentUser;
  authorization: AuthorizationSummary;
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

export type GetCurrentUserInput = {
  userSubject: string;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export type UpdateCurrentUserInput = {
  userSubject: string;
  displayName: string;
  timezone: string;
  weatherLocationEnabled: boolean;
  weatherLocationSource?: string;
  weatherLocationLabel?: string;
  weatherLatitude?: number;
  weatherLongitude?: number;
  hasWeatherLocation: boolean;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export type RefreshCurrentUserIdentityInput = {
  userSubject: string;
  email?: string;
  emailVerified?: boolean;
  displayName?: string;
  avatarUrl?: string;
  givenName?: string;
  familyName?: string;
  locale?: string;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export interface ControlPlaneClient {
  getCurrentUser(input: GetCurrentUserInput): Promise<CurrentUserBootstrap>;
  updateCurrentUser(input: UpdateCurrentUserInput): Promise<CurrentUser>;
  refreshCurrentUserIdentity(input: RefreshCurrentUserIdentityInput): Promise<CurrentUser>;
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
  GetCurrentUser: GrpcUnaryMethod;
  UpdateCurrentUser: GrpcUnaryMethod;
  RefreshCurrentUserIdentity: GrpcUnaryMethod;
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

type RawCurrentUser = Partial<Record<keyof CurrentUser, unknown>>;
type RawAuthorizationSummary = Partial<Record<keyof AuthorizationSummary, unknown>>;
type RawGetCurrentUserResponse = {
  user?: unknown;
  authorization?: unknown;
};
type RawUpdateCurrentUserResponse = {
  user?: unknown;
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
    async getCurrentUser(input) {
      const response = await unaryCall<RawGetCurrentUserResponse>(
        client.GetCurrentUser.bind(client),
        { userSubject: input.userSubject },
        input
      );
      return {
        user: normalizeCurrentUser((response.user ?? {}) as RawCurrentUser),
        authorization: normalizeAuthorizationSummary(
          (response.authorization ?? {}) as RawAuthorizationSummary
        )
      };
    },
    async updateCurrentUser(input) {
      const response = await unaryCall<RawUpdateCurrentUserResponse>(
        client.UpdateCurrentUser.bind(client),
        {
          userSubject: input.userSubject,
          displayName: input.displayName,
          timezone: input.timezone,
          weatherLocationEnabled: input.weatherLocationEnabled,
          weatherLocationSource: input.weatherLocationSource ?? '',
          weatherLocationLabel: input.weatherLocationLabel ?? '',
          weatherLatitude: input.weatherLatitude ?? 0,
          weatherLongitude: input.weatherLongitude ?? 0,
          hasWeatherLocation: input.hasWeatherLocation
        },
        input
      );
      return normalizeCurrentUser((response.user ?? {}) as RawCurrentUser);
    },
    async refreshCurrentUserIdentity(input) {
      const response = await unaryCall<RawUpdateCurrentUserResponse>(
        client.RefreshCurrentUserIdentity.bind(client),
        {
          userSubject: input.userSubject,
          email: input.email ?? '',
          emailVerified: input.emailVerified ?? false,
          displayName: input.displayName ?? '',
          avatarUrl: input.avatarUrl ?? '',
          givenName: input.givenName ?? '',
          familyName: input.familyName ?? '',
          locale: input.locale ?? ''
        },
        input
      );
      return normalizeCurrentUser((response.user ?? {}) as RawCurrentUser);
    },
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

function normalizeCurrentUser(user: RawCurrentUser): CurrentUser {
  return {
    id: normalizeString(user.id),
    keycloakSubject: normalizeString(user.keycloakSubject),
    email: normalizeString(user.email),
    emailVerified: Boolean(user.emailVerified),
    displayName: normalizeString(user.displayName),
    displayNameSource: normalizeString(user.displayNameSource),
    avatarUrl: normalizeString(user.avatarUrl),
    givenName: normalizeString(user.givenName),
    familyName: normalizeString(user.familyName),
    locale: normalizeString(user.locale),
    timezone: normalizeString(user.timezone),
    weatherLocationEnabled: Boolean(user.weatherLocationEnabled),
    weatherLocationSource: normalizeString(user.weatherLocationSource),
    weatherLocationLabel: normalizeString(user.weatherLocationLabel),
    weatherLatitude: normalizeNumber(user.weatherLatitude),
    weatherLongitude: normalizeNumber(user.weatherLongitude),
    hasWeatherLocation: Boolean(user.hasWeatherLocation),
    lastLoginAtUnixMs: normalizeString(user.lastLoginAtUnixMs),
    createdAtUnixMs: normalizeString(user.createdAtUnixMs),
    updatedAtUnixMs: normalizeString(user.updatedAtUnixMs),
    authMethod: normalizeString(user.authMethod)
  };
}

function normalizeAuthorizationSummary(summary: RawAuthorizationSummary): AuthorizationSummary {
  return {
    tokenRoles: Array.isArray(summary.tokenRoles)
      ? summary.tokenRoles.filter((value): value is string => typeof value === 'string' && value.trim().length > 0)
      : [],
    deviceCount: normalizeInteger(summary.deviceCount)
  };
}

function normalizeString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}

function normalizeNumber(value: unknown): number | undefined {
  return typeof value === 'number' && Number.isFinite(value) ? value : undefined;
}

function normalizeInteger(value: unknown): number {
  if (typeof value === 'number' && Number.isFinite(value)) {
    return Math.max(0, Math.trunc(value));
  }
  if (typeof value === 'string' && value.trim()) {
    const parsed = Number(value);
    if (Number.isFinite(parsed)) {
      return Math.max(0, Math.trunc(parsed));
    }
  }
  return 0;
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
