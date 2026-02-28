import path from 'node:path';
import { fileURLToPath } from 'node:url';

import grpc from '@grpc/grpc-js';
import protoLoader from '@grpc/proto-loader';

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
const controlPlaneProto = grpc.loadPackageDefinition(packageDefinition) as unknown as {
  pulse: {
    controlplane: {
      v1: {
        ControlPlaneService: new (
          address: string,
          credentials: grpc.ChannelCredentials,
          options?: Record<string, unknown>
        ) => ControlPlaneClient;
      };
    };
  };
};

type ControlPlaneClient = {
  ListUserDevices: (
    request: { userSubject?: string },
    metadata: grpc.Metadata,
    options: grpc.CallOptions,
    callback: (error: grpc.ServiceError | null, response?: { devices?: Array<{ deviceId?: string }> }) => void
  ) => void;
  close: () => void;
};

export interface DeviceAuthorizer {
  authorize(input: { deviceId: string; authHeader?: string; requestID?: string; deadlineMs: number }): Promise<void>;
  close(): void;
}

export function createPermissiveDeviceAuthorizer(): DeviceAuthorizer {
  return {
    async authorize(): Promise<void> {},
    close(): void {}
  };
}

export function createControlPlaneDeviceAuthorizer(address: string, defaultUserSubject?: string): DeviceAuthorizer {
  const client = new controlPlaneProto.pulse.controlplane.v1.ControlPlaneService(
    address,
    grpc.credentials.createInsecure()
  );

  return {
    authorize(input) {
      return new Promise<void>((resolve, reject) => {
        const metadata = new grpc.Metadata();
        if (input.authHeader) {
          metadata.set('authorization', input.authHeader);
        }
        if (input.requestID) {
          metadata.set('x-request-id', input.requestID);
        }
        const request = {
          userSubject: input.authHeader ? undefined : defaultUserSubject
        };
        client.ListUserDevices(
          request,
          metadata,
          { deadline: new Date(Date.now() + input.deadlineMs) },
          (error, response) => {
            if (error) {
              reject(error as Error & { code?: number });
              return;
            }
            const allowed = new Set((response?.devices ?? []).map((device) => String(device.deviceId ?? '').trim()).filter(Boolean));
            if (allowed.has(input.deviceId)) {
              resolve();
              return;
            }
            reject(Object.assign(new Error('device access denied'), { code: grpc.status.PERMISSION_DENIED }));
          }
        );
      });
    },
    close() {
      client.close();
    }
  };
}
