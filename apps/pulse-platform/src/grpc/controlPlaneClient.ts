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

export type ListUserDevicesInput = {
  userSubject: string;
  authHeader?: string;
  requestID?: string;
  deadlineMs: number;
};

export interface ControlPlaneClient {
  listUserDevices(input: ListUserDevicesInput): Promise<UserDevice[]>;
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

function normalizeString(value: unknown): string {
  return typeof value === 'string' ? value : '';
}
