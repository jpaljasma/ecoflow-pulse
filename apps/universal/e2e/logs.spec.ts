import { expect, test, type Page } from '@playwright/test';
import { mockApiRoutes } from './mockApi';

test.describe('Universal admin logs', () => {
  test('hides Logs navigation and guards direct route access for users without devices', async ({ page }) => {
    await mockApiRoutes(page, { deviceCount: 0 });
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto('/devices');
    await expect(page.getByTestId('sidebar-logs')).toHaveCount(0);

    await page.goto('/logs');
    await expect(page.getByTestId('screen-logs-forbidden')).toBeVisible();
    await expect(page.getByText('Logs unavailable')).toBeVisible();
  });

  test('shows owner-scoped logs without user lookup for non-admin device owners', async ({ page }) => {
    await mockApiRoutes(page, { roles: ['viewer'], deviceCount: 2 });
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto('/logs');

    await expect(page.getByTestId('screen-logs')).toBeVisible();
    await expect(page.getByLabel('Email')).toHaveCount(0);
    await expect(page.getByLabel('Provider')).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Device' })).toBeVisible();
    await expect(page.getByRole('textbox', { name: 'Serial' })).toBeVisible();

    await page.getByRole('textbox', { name: 'Serial' }).fill('DPU');
    await expect(page.getByTestId('logs-typeahead-menu-serial').getByText('DPU A 12 kWh')).toBeVisible();
    await expect(page.getByText('operator@example.invalid')).toHaveCount(0);
  });

  test('shows realtime logs for admins, reconnects, and supports POST typeahead filters', async ({ page }) => {
    await mockApiRoutes(page, { roles: ['viewer', 'admin'] });
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto('/logs');

    await expect(page.getByTestId('screen-logs')).toBeVisible();
    await expect(page.getByText('Realtime MQTT operations console')).toBeVisible();
    await expect(page.getByText('Live')).toBeVisible({ timeout: 5000 });
    await expect(page.getByText(/quota .* frame/i).first()).toBeVisible();
    await expect(page.getByText(/DPU A 12 kWh <11111111\.\.\.1111>/).first()).toBeVisible();
    await expect(page.getByText('op***@ex***').first()).toBeVisible();
    await expect(page.getByText('operator@example.invalid')).toHaveCount(0);

    await page.getByLabel('Email').fill('operator');
    await expect(page.getByText('operator@example.invalid')).toBeVisible();
    await page.getByText('operator@example.invalid').click();
    expect(new URL(page.url()).search).toBe('');

    await page.getByLabel('Freetext fuzzy search').fill('quota');
    await expect(page.getByText(/quota .* frame/i).first()).toBeVisible();
    await page.getByRole('button', { name: /pause/i }).click();
    await page.getByText(/quota .* frame/i).first().click();
    await expect(page.getByText('"payload"')).toBeVisible();
    await page.getByText(/quota .* frame/i).nth(1).click();
    await expect(page.getByText('"payload"')).toHaveCount(1);
  });

  test('applies canonical device deep links to websocket log filters', async ({ page }) => {
    await captureWebSocketMessages(page);
    await mockApiRoutes(page, { roles: ['viewer', 'admin'] });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/logs?deviceId=11111111-1111-7111-8111-111111111111');

    await expect(page.getByText('Live')).toBeVisible({ timeout: 5000 });
    await expect.poll(() =>
      page.evaluate(() => {
        const messages = (window as unknown as { __pulseWsSentMessages?: Array<{ type?: string; filters?: { deviceIds?: string[] } }> }).__pulseWsSentMessages ?? [];
        return messages.find((message) => message.type === 'logs_subscribe')?.filters?.deviceIds ?? [];
      })
    ).toEqual(['11111111-1111-7111-8111-111111111111']);
  });

  test('treats status filters as an exclusive toggle', async ({ page }) => {
    await captureWebSocketMessages(page);
    await mockApiRoutes(page, { roles: ['viewer', 'admin'] });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/logs');
    await expect(page.getByText('Live')).toBeVisible({ timeout: 5000 });

    await page.getByRole('button', { name: 'Enable OK filter' }).click();
    await expect.poll(() => latestLogSubscribeStatuses(page)).toEqual(['ok']);

    await page.getByRole('button', { name: 'Enable Warn filter' }).click();
    await expect.poll(() => latestLogSubscribeStatuses(page)).toEqual(['warning']);

    await page.getByRole('button', { name: 'Disable Warn filter' }).click();
    await expect.poll(() => latestLogSubscribeStatuses(page)).toEqual([]);
  });

  test('subscribes Status and Info type filters as suffix families', async ({ page }) => {
    await captureWebSocketMessages(page);
    await mockApiRoutes(page, { roles: ['viewer', 'admin'] });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/logs');
    await expect(page.getByText('Live')).toBeVisible({ timeout: 5000 });

    await page.getByRole('button', { name: 'Enable Status filter' }).click();
    await expect.poll(() => latestLogSubscribeTypeFilter(page)).toEqual({
      typeCodes: [],
      typeCodeSuffixes: ['Status']
    });

    await page.getByRole('button', { name: 'Enable Info filter' }).click();
    await expect.poll(() => latestLogSubscribeTypeFilter(page)).toEqual({
      typeCodes: [],
      typeCodeSuffixes: ['Info']
    });
  });

  test('keeps typeahead suggestions floating above the filter grid', async ({ page }) => {
    await mockApiRoutes(page, { roles: ['viewer', 'admin'] });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/logs');
    await expect(page.getByTestId('screen-logs')).toBeVisible();

    const tableTopBefore = await page.getByTestId('logs-table').evaluate((node) => node.getBoundingClientRect().top);
    await page.getByLabel('Email').fill('operator');
    await expect(page.getByTestId('logs-typeahead-menu-user')).toBeVisible();
    const tableTopAfter = await page.getByTestId('logs-table').evaluate((node) => node.getBoundingClientRect().top);

    expect(Math.abs(tableTopAfter - tableTopBefore)).toBeLessThanOrEqual(2);
  });

  test('limits device and serial typeahead results to the selected provider', async ({ page }) => {
    await mockApiRoutes(page, { roles: ['viewer', 'admin'] });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/logs');
    await expect(page.getByTestId('screen-logs')).toBeVisible();

    await page.getByLabel('Provider').selectOption('pecron');
    await page.getByRole('textbox', { name: 'Device' }).fill('pack');
    await expect(page.getByTestId('logs-typeahead-menu-device').getByText('Pecron balcony pack')).toBeVisible();

    await page.getByRole('textbox', { name: 'Device' }).fill('DPU');
    await expect(page.getByTestId('logs-typeahead-menu-device')).toHaveCount(0);

    await page.getByRole('textbox', { name: 'Serial' }).fill('DEMO');
    await expect(page.getByTestId('logs-typeahead-menu-serial').getByText('P11VXG:DEMO-001')).toBeVisible();
  });

  test('clears visible logs while paused and keeps the buffer empty', async ({ page }) => {
    await mockApiRoutes(page, { roles: ['viewer', 'admin'] });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/logs');
    await expect(page.getByText('Live')).toBeVisible({ timeout: 5000 });
    await expect(page.getByText(/frame for/i).first()).toBeVisible();

    await page.getByRole('button', { name: /pause/i }).click();
    await page.getByRole('button', { name: /clear/i }).click();

    await expect(page.getByText(/frame for/i)).toHaveCount(0);
    await expect(page.getByText('Waiting for matching log entries')).toBeVisible();
  });

  test('does not receive log entries while another tab is active and resumes on return', async ({ page }) => {
    await page.addInitScript(() => {
      const NativeWebSocket = window.WebSocket;
      const counters = { offTabLogEntries: 0 };
      (window as unknown as { __pulseLogCounters: typeof counters }).__pulseLogCounters = counters;
      window.WebSocket = class extends NativeWebSocket {
        constructor(url: string | URL, protocols?: string | string[]) {
          super(url, protocols);
          this.addEventListener('message', (event) => {
            try {
              const message = JSON.parse(String(event.data)) as { type?: string };
              if (message.type === 'log_entry' && window.location.pathname !== '/logs') {
                counters.offTabLogEntries += 1;
              }
            } catch {
              // Ignore non-log frames in the test transport counter.
            }
          });
        }
      };
    });
    await mockApiRoutes(page, { roles: ['viewer', 'admin'] });
    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/logs');
    await expect(page.getByText('Live')).toBeVisible({ timeout: 5000 });
    await expect(page.getByText(/frame for/i).first()).toBeVisible();

    await page.goto('/devices');
    await page.waitForTimeout(2200);

    await expect.poll(() => page.evaluate(() => (
      window as unknown as { __pulseLogCounters?: { offTabLogEntries: number } }
    ).__pulseLogCounters?.offTabLogEntries ?? 0)).toBe(0);

    await page.goto('/logs');
    await expect(page.getByText('Live')).toBeVisible({ timeout: 5000 });
  });
});

async function captureWebSocketMessages(page: Page): Promise<void> {
  await page.addInitScript(() => {
    const NativeWebSocket = window.WebSocket;
    const sentMessages: unknown[] = [];
    (window as unknown as { __pulseWsSentMessages: unknown[] }).__pulseWsSentMessages = sentMessages;
    window.WebSocket = class extends NativeWebSocket {
      send(data: string | ArrayBufferLike | Blob | ArrayBufferView): void {
        if (typeof data === 'string') {
          try {
            sentMessages.push(JSON.parse(data) as unknown);
          } catch {
            // Ignore non-JSON websocket frames in the capture helper.
          }
        }
        super.send(data);
      }
    };
  });
}

async function latestLogSubscribeStatuses(page: Page): Promise<string[]> {
  return page.evaluate(() => {
    const messages = (
      window as unknown as { __pulseWsSentMessages?: Array<{ type?: string; filters?: { statuses?: string[] } }> }
    ).__pulseWsSentMessages ?? [];
    return [...messages].reverse().find((message) => message.type === 'logs_subscribe')?.filters?.statuses ?? [];
  });
}

async function latestLogSubscribeTypeFilter(page: Page): Promise<{ typeCodes: string[]; typeCodeSuffixes: string[] }> {
  return page.evaluate(() => {
    const messages = (
      window as unknown as {
        __pulseWsSentMessages?: Array<{
          type?: string;
          filters?: { typeCodes?: string[]; typeCodeSuffixes?: string[] };
        }>;
      }
    ).__pulseWsSentMessages ?? [];
    const filters = [...messages].reverse().find((message) => message.type === 'logs_subscribe')?.filters;
    return {
      typeCodes: filters?.typeCodes ?? [],
      typeCodeSuffixes: filters?.typeCodeSuffixes ?? []
    };
  });
}
