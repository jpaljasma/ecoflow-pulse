import { expect, test } from '@playwright/test';
import { mockApiRoutes } from './mockApi';

test.describe('Integration settings web E2E', () => {
  test.beforeEach(async ({ page }) => {
    await mockApiRoutes(page);
  });

  test('renders every provider connector in the catalog', async ({ page }) => {
    await page.goto('/settings/integrations');

    await expect(page.getByText('EcoFlow', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Pulse MQTT Emulator', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Pecron', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Anker SOLIX Cloud MQTT', { exact: true }).first()).toBeVisible();

    await page.getByRole('button', { name: /Pecron/ }).click();
    await expect(page.getByText('Configure Pecron', { exact: true })).toBeVisible();
    await expect(page.getByText('Cloud region', { exact: true })).toBeVisible();

    await page.getByRole('button', { name: /Anker SOLIX Cloud MQTT/ }).click();
    await expect(page.getByText('Configure Anker SOLIX', { exact: true })).toBeVisible();
    await expect(page.getByText('Cloud server', { exact: true })).toBeVisible();
    await expect(page.getByText('Account country', { exact: true })).toBeVisible();
  });
});
