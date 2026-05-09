import { expect, test } from '@playwright/test';
import { mockApiRoutes } from './mockApi';

async function expandSolarAgainstLoadControls(page: Parameters<typeof test.beforeEach>[0]['page']) {
  const scopeHeading = page.getByText('Scope', { exact: true });
  if (await scopeHeading.count()) {
    await expect(scopeHeading).toBeVisible();
    return;
  }

  const expandButton = page.getByLabel('Expand solar against load controls');
  if (await expandButton.count()) {
    await expandButton.click({ force: true });
    await expect(scopeHeading).toBeVisible();
    return;
  }

  await expect(scopeHeading).toBeVisible();
}

test.describe('Energy route web E2E', () => {
  test.describe.configure({ mode: 'serial' });

  test.beforeEach(async ({ page }) => {
    await mockApiRoutes(page);
  });

  test('renders the energy dashboard from API responses', async ({ page }) => {
    await page.goto('/energy');

    await expect(page.getByText('Solar against load', { exact: true })).toBeVisible();
    await expect(page.getByText('Storm Guard active for ~2h more', { exact: true })).toBeVisible();
    await expect(page.getByText('Estimated value', { exact: true })).toBeVisible();
    await expect(page.getByText('State of Energy', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Panel', { exact: true })).toBeVisible();
    await expect(page.getByRole('button', { name: 'Impact' })).toBeVisible();
    await expect(page.getByText('More solar freedom', { exact: true })).toBeVisible();
    await expect(page.getByText('Power profile', { exact: true })).toBeVisible();
    await expect(page.getByText('Energy balance', { exact: true })).toBeVisible();
    await expect(page.getByText('PV operating envelope', { exact: true })).toBeVisible();
    await expect(page.getByText('Battery flow', { exact: true })).toBeVisible();
    await expect(page.getByText('Flow strip', { exact: true })).toBeVisible();
    await expect(page.getByText('SOC band', { exact: true })).toBeVisible();

    await expandSolarAgainstLoadControls(page);
    await expect(page.getByRole('button', { name: 'All devices' })).toBeVisible();
    await expect(page.getByText('Battery', { exact: true })).toBeVisible();
    await expect(page.getByText('Current points: 3 · Previous points: 2', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Grid in', { exact: true })).toBeVisible();
    await expect(page.getByText('AC out', { exact: true })).toBeVisible();
    await expect(page.getByText('DC out', { exact: true })).toBeVisible();
    await expect(page.getByText('Charge', { exact: true })).toBeVisible();
    await expect(page.getByText('Discharge', { exact: true })).toBeVisible();
    await expect(
      page.getByText('Using minute buckets in America/New_York for current-period diagnostics.', { exact: true })
    ).toBeVisible();
    await expect(page.getByText('DPU A 12 kWh', { exact: true }).nth(1)).toBeVisible();
    await expect(page.getByText('PV Low', { exact: false }).first()).toBeVisible();
    await expect(page.getByText('Observed', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Within envelope', { exact: true }).first()).toBeVisible();
  });

  test('updates the route and history counts when comparison is disabled', async ({ page }) => {
    await page.goto('/energy?compare=0');

    await expect(page).toHaveURL(/compare=0/);
    await expect(page.getByText('Current points: 3', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Previous points:', { exact: false })).toHaveCount(0);
    await expect(page.getByText('No prior baseline', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Previous', { exact: true })).toHaveCount(0);
  });

  test('switches to single-device mode and renders scoped PV history', async ({ page }) => {
    await page.goto('/energy?device=22222222-2222-7222-8222-222222222222');

    await expect(page).toHaveURL(/device=22222222-2222-7222-8222-222222222222/);
    await expect(page.getByText('PV 1', { exact: true })).toBeVisible();
    await expect(page.getByText('Historical PV observations are not available for this port in the selected window.', { exact: true })).toBeVisible();
    await expect(page.getByText('Hist max W', { exact: true })).toBeVisible();
  });

  test('switches presets and keeps the spec-aligned route params', async ({ page }) => {
    await page.goto('/energy?preset=thisMonth');

    await expect(page).toHaveURL(/preset=thisMonth/);
    await expect(page.getByText('Using hour buckets in America/New_York for current-period diagnostics.', { exact: true })).toBeVisible();
    await expect(page.getByText('America/New_York', { exact: true })).toBeVisible();
  });
});
