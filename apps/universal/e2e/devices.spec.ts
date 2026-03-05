import { expect, test } from '@playwright/test';
import {
  D2M_DEVICE_ID,
  D2M_SERIAL,
  DPU_DEVICE_ID,
  mockApiRoutes
} from './mockApi';

test.describe('Universal web E2E', () => {
  test.beforeEach(async ({ page }) => {
    await mockApiRoutes(page);
  });

  test('renders the devices page from API responses', async ({ page }) => {
    await page.goto('/devices');

    await expect(page.getByText('Fleet Summary')).toBeVisible();
    await expect(page.getByText('DPU A 12 kWh')).toBeVisible();
    await expect(page.getByText('Kitchen Delta 2 Max')).toBeVisible();
    await expect(page.getByText('☼ Today').first()).toBeVisible();
  });

  test('loads detail by UUID route', async ({ page }) => {
    await page.goto(`/device/${DPU_DEVICE_ID}`);

    await expect(page).toHaveURL(new RegExp(`/device/${DPU_DEVICE_ID}$`));
    await expect(page.getByText('DPU A 12 kWh')).toBeVisible();
    await expect(page.getByText('Battery Packs')).toBeVisible();
    await expect(page.getByText('Solar Inputs')).toBeVisible();
    await expect(page.getByText('System Signals')).toBeVisible();
  });

  test('resolves serial route to canonical UUID route', async ({ page }) => {
    await page.goto(`/device/${D2M_SERIAL}`);

    await expect(page).toHaveURL(new RegExp(`/device/${D2M_DEVICE_ID}$`));
    await expect(page.getByText('Kitchen Delta 2 Max')).toBeVisible();
    await expect(page.getByText('☼ Solar Generated (6am-6pm, 10m buckets)')).toBeVisible();
  });
});
