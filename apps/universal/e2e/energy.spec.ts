import { expect, test } from '@playwright/test';
import { mockApiRoutes } from './mockApi';

test.describe('Energy route web E2E', () => {
  test.beforeEach(async ({ page }) => {
    await mockApiRoutes(page);
  });

  test('renders the energy dashboard from API responses', async ({ page }) => {
    await page.goto('/energy?tz=UTC');

    await expect(page.getByText('Solar against load', { exact: true })).toBeVisible();
    await expect(page.getByText('Estimated value', { exact: true })).toBeVisible();
    await expect(page.getByText('Power profile', { exact: true })).toBeVisible();
    await expect(page.getByText('Energy history', { exact: true })).toBeVisible();
    await expect(page.getByText('PV operating envelope', { exact: true })).toBeVisible();
    await expect(page.getByText('Battery flow', { exact: true })).toBeVisible();
    await expect(page.getByText('Flow strip', { exact: true })).toBeVisible();
    await expect(page.getByText('SOC band', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'All devices' })).toBeVisible();
    await expect(page.getByText('Battery', { exact: true })).toBeVisible();
    await expect(page.getByText('Current period uses solid lines. Previous period uses lighter overlays.', { exact: true })).toBeVisible();
    await expect(page.getByText('Current points: 3 · Previous points: 2', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('AC input', { exact: true })).toBeVisible();
    await expect(page.getByText('AC output', { exact: true })).toBeVisible();
    await expect(page.getByText('DC output', { exact: true })).toBeVisible();
    await expect(page.getByText('Battery charge', { exact: true })).toBeVisible();
    await expect(page.getByText('Battery discharge', { exact: true })).toBeVisible();
    await expect(
      page.getByText('Using minute buckets in UTC for current-period diagnostics.', { exact: true })
    ).toBeVisible();
    await expect(page.getByText('DPU A 12 kWh', { exact: true }).nth(1)).toBeVisible();
    await expect(page.getByText('PV Low', { exact: false }).first()).toBeVisible();
    await expect(page.getByText('Observed', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Within envelope', { exact: true }).first()).toBeVisible();
  });

  test('updates the route and history counts when comparison is disabled', async ({ page }) => {
    await page.goto('/energy?tz=UTC');

    await page.getByRole('button', { name: 'Compare off' }).click();

    await expect(page).toHaveURL(/compare=0/);
    await expect(page.getByText('Current points: 3', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Previous points:', { exact: false })).toHaveCount(0);
    await expect(page.getByText('No prior baseline', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Previous', { exact: true })).toHaveCount(0);
  });

  test('switches to single-device mode and renders scoped PV history', async ({ page }) => {
    await page.goto('/energy?tz=UTC');

    await page.getByRole('button', { name: 'Kitchen Delta 2 Max' }).click();

    await expect(page).toHaveURL(/device=22222222-2222-7222-8222-222222222222/);
    await expect(page.getByText('Kitchen Delta 2 Max · Mar 3 - Mar 3', { exact: true })).toBeVisible();
    await expect(page.getByText('PV 1', { exact: true })).toBeVisible();
    await expect(page.getByText('Historical PV observations are not available for this port in the selected window.', { exact: true })).toBeVisible();
    await expect(page.getByText('Hist max W', { exact: true })).toBeVisible();
  });

  test('switches presets and keeps the spec-aligned route params', async ({ page }) => {
    await page.goto('/energy?tz=UTC');

    await page.getByRole('button', { name: 'This month' }).click();

    await expect(page).toHaveURL(/preset=thisMonth/);
    await expect(page.getByText('Using hour buckets in UTC for current-period diagnostics.', { exact: true })).toBeVisible();
    await expect(page.getByText('UTC', { exact: true })).toBeVisible();
  });
});
