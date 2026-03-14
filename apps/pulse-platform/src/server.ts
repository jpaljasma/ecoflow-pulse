import httpProxy from 'http-proxy';

import { buildApp } from './app.js';
import { loadConfig } from './config.js';
import { createControlPlaneClient } from './grpc/controlPlaneClient.js';
import { createDeviceClient } from './grpc/deviceClient.js';
import { createInferenceClient } from './grpc/inferenceClient.js';
import { createTelemetryHistoryClient, createTelemetrySnapshotClient } from './grpc/telemetryClient.js';

const config = loadConfig(process.env);
const historyClient = createTelemetryHistoryClient(config.energyGrpcApiAddr);
const controlPlaneClient = createControlPlaneClient(config.grpcApiAddr);
const currentUserClient = createControlPlaneClient(config.grpcApiAddr);
const snapshotClient = createTelemetrySnapshotClient(config.grpcApiAddr);
const deviceClient = createDeviceClient(config, controlPlaneClient, snapshotClient);
const inferenceClient = createInferenceClient(config.grpcApiAddr);
const app = buildApp(config, historyClient, deviceClient, inferenceClient, {
  controlPlaneClient: currentUserClient
});
const wsProxy = config.realtimeGatewayUpstreamUrl
  ? httpProxy.createProxyServer({
      target: config.realtimeGatewayUpstreamUrl,
      ws: true,
      changeOrigin: true,
      xfwd: true
    })
  : null;

if (wsProxy) {
  wsProxy.on('error', (error, _req, socket) => {
    app.log.warn({ error }, 'realtime gateway websocket proxy error');
    if (socket && 'destroy' in socket) {
      socket.destroy();
    }
  });
  app.server.on('upgrade', (request, socket, head) => {
    if (!request.url?.startsWith('/ws')) {
      socket.destroy();
      return;
    }
    wsProxy.ws(request, socket, head);
  });
}

const shutdown = async (signal: string) => {
  app.log.info({ signal }, 'shutting down pulse-platform');
  wsProxy?.close();
  await app.close();
  process.exit(0);
};

process.on('SIGINT', () => void shutdown('SIGINT'));
process.on('SIGTERM', () => void shutdown('SIGTERM'));

try {
  await app.listen({ host: config.host, port: config.port });
} catch (error) {
  console.error(error);
  app.log.error(error);
  process.exit(1);
}
