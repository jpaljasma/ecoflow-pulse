import { expect, test } from '@playwright/test';
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
    await expect(page.getByText('DPU A 12 kWh')).toBeVisible();
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
    await expect(page.getByText('Garage battery').first()).toBeVisible();
    await expect(page.getByText('<redacted>').first()).toBeVisible();

    await page.getByLabel('Email').fill('operator');
    await expect(page.getByText('operator@example.invalid')).toBeVisible();
    await page.getByText('operator@example.invalid').click();
    expect(new URL(page.url()).search).toBe('');

    await page.getByLabel('Freetext fuzzy search').fill('quota');
    await expect(page.getByText(/quota .* frame/i).first()).toBeVisible();
    await page.getByText(/quota .* frame/i).first().click();
    await expect(page.getByText('"payload"')).toBeVisible();
    await page.getByText(/quota .* frame/i).nth(1).click();
    await expect(page.getByText('"payload"')).toHaveCount(1);
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
});
