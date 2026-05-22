import { expect, test } from '@playwright/test';
import { mockApiRoutes } from './mockApi';

test.describe('Universal admin logs', () => {
  test('hides Logs navigation and guards direct route access for non-admin users', async ({ page }) => {
    await mockApiRoutes(page);
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto('/devices');
    await expect(page.getByTestId('sidebar-logs')).toHaveCount(0);

    await page.goto('/logs');
    await expect(page.getByTestId('screen-logs-forbidden')).toBeVisible();
    await expect(page.getByText('Admin access required')).toBeVisible();
  });

  test('shows realtime logs for admins with POST typeahead filters and row drilldown', async ({ page }) => {
    await mockApiRoutes(page, { roles: ['viewer', 'admin'] });
    await page.setViewportSize({ width: 1440, height: 900 });

    await page.goto('/logs');

    await expect(page.getByTestId('screen-logs')).toBeVisible();
    await expect(page.getByText('Realtime MQTT operations console')).toBeVisible();
    await expect(page.getByText('Live')).toBeVisible({ timeout: 5000 });
    await expect(page.getByText(/quota .* frame/i).first()).toBeVisible();

    await page.getByLabel('User email').fill('operator');
    await expect(page.getByText('operator@example.invalid')).toBeVisible();
    await page.getByText('operator@example.invalid').click();
    expect(new URL(page.url()).search).toBe('');

    await page.getByLabel('Freetext fuzzy search').fill('quota');
    await expect(page.getByText(/quota .* frame/i).first()).toBeVisible();
    await page.getByText(/quota .* frame/i).first().click();
    await expect(page.getByText('"payload"')).toBeVisible();
  });
});
