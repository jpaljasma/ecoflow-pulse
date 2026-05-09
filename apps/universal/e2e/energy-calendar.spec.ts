import { expect, test } from '@playwright/test';
import { mockApiRoutes } from './mockApi';

test.describe('Energy Calendar route web E2E', () => {
  test.beforeEach(async ({ page }) => {
    await mockApiRoutes(page);
  });

  test('renders the calendar and routes a day to Energy with a selected date', async ({ page }) => {
    await page.goto('/energy-calendar?timezone=UTC&year=2026&month=3');

    await expect(page.getByText('Energy Calendar', { exact: true })).toBeVisible();
    await expect(page.getByText('March 2026', { exact: true })).toBeVisible();
    await expect(page.getByText('solar', { exact: true })).toBeVisible();
    await expect(page.getByText('saved', { exact: true })).toBeVisible();
    await expect(page.getByTestId('energy-calendar-device-select')).toBeVisible();
    await expect(page.getByText('Sun', { exact: true })).toBeVisible();

    await page.getByTestId('energy-calendar-day-2026-03-04').click();
    await expect(page).toHaveURL(/\/energy\?/);
    await expect(page).toHaveURL(/date=2026-03-04/);
  });
});
