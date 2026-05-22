import { defineConfig, devices } from '@playwright/test';

const webPort = Number(process.env.PLAYWRIGHT_WEB_PORT ?? '4173');
const exportScript = process.env.CI ? 'export:web:ci' : 'export:web';

export default defineConfig({
  testDir: './e2e',
  timeout: 30_000,
  expect: {
    timeout: 8_000
  },
  fullyParallel: true,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI
    ? [['github'], ['html', { open: 'never' }]]
    : [['list'], ['html', { open: 'never' }]],
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? `http://127.0.0.1:${webPort}`,
    trace: 'retain-on-failure',
    screenshot: 'only-on-failure',
    video: 'retain-on-failure'
  },
  webServer: [
    {
      command: `CI=1 EXPO_PUBLIC_WS_URL=ws://127.0.0.1:8082/ws npm run ${exportScript} && npx serve -s dist -l ${webPort}`,
      port: webPort,
      timeout: 240_000,
      reuseExistingServer: !process.env.CI
    },
    {
      command: 'MAESTRO_MOCK_WS_CLOSE_LOGS_ONCE=1 node e2e/mock_ws_server.js',
      port: 8082,
      timeout: 30_000,
      reuseExistingServer: !process.env.CI
    }
  ],
  projects: [
    {
      name: 'chromium',
      use: {
        ...devices['Desktop Chrome']
      }
    }
  ]
});
