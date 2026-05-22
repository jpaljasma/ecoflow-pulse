import { buildApp } from './app.js';
import { loadConfig } from './config.js';
import {
  createControlPlaneDeviceAuthorizer,
  createPermissiveDeviceAuthorizer
} from './controlplane/deviceAuthorizer.js';
import { createLiveTelemetryClient } from './live/liveTelemetryClient.js';
import { NatsDeltaHub } from './live/natsDeltaHub.js';
import { NatsAdminLogSource } from './adminLogs/natsAdminLogSource.js';
import { ValkeySnapshotStore } from './snapshot/valkeySnapshotStore.js';

const config = loadConfig(process.env);
const authorizer =
  config.auth.mode === 'noop'
    ? createPermissiveDeviceAuthorizer()
    : createControlPlaneDeviceAuthorizer(config.grpcApiAddr);
const liveClient = createLiveTelemetryClient({
  authorizer,
  snapshots: new ValkeySnapshotStore(config.valkey),
  deltaHub: new NatsDeltaHub({
    urls: config.natsUrls,
    subjectPrefix: config.telemetrySubjectPrefix
  })
});
const app = buildApp(config, liveClient, {
  deviceAuthorizer: authorizer,
  logSource: new NatsAdminLogSource({
    urls: config.natsUrls,
    subjectPrefix: config.telemetrySubjectPrefix,
    streamName: config.logs.streamName
  })
});

const shutdown = async (signal: string) => {
  app.log.info({ signal }, 'shutting down pulse-realtime-gateway');
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
