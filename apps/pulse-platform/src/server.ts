import { buildApp } from './app.js';
import { loadConfig } from './config.js';
import { createControlPlaneClient } from './grpc/controlPlaneClient.js';
import { createDeviceClient } from './grpc/deviceClient.js';
import { createTelemetryHistoryClient, createTelemetrySnapshotClient } from './grpc/telemetryClient.js';

const config = loadConfig(process.env);
const historyClient = createTelemetryHistoryClient(config.grpcApiAddr);
const controlPlaneClient = createControlPlaneClient(config.grpcApiAddr);
const snapshotClient = createTelemetrySnapshotClient(config.grpcApiAddr);
const deviceClient = createDeviceClient(config, controlPlaneClient, snapshotClient);
const app = buildApp(config, historyClient, deviceClient);

const shutdown = async (signal: string) => {
  app.log.info({ signal }, 'shutting down pulse-platform');
  await app.close();
  process.exit(0);
};

process.on('SIGINT', () => void shutdown('SIGINT'));
process.on('SIGTERM', () => void shutdown('SIGTERM'));

try {
  await app.listen({ host: config.host, port: config.port });
} catch (error) {
  app.log.error(error);
  process.exit(1);
}
