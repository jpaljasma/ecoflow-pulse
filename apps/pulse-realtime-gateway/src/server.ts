import { buildApp } from './app.js';
import { loadConfig } from './config.js';
import { createLiveTelemetryClient } from './grpc/liveTelemetryClient.js';

const config = loadConfig(process.env);
const liveClient = createLiveTelemetryClient(config.grpcApiAddr);
const app = buildApp(config, liveClient);

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
