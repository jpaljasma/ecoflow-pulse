import { expect, test } from '@playwright/test';
import { mockApiRoutes } from './mockApi';

test.describe('Energy Calendar route web E2E', () => {
  test.beforeEach(async ({ page }) => {
    await mockApiRoutes(page);
  });

  test('renders the calendar and routes a day to Energy with a selected date', async ({ page }) => {
    await page.goto('/energy-calendar?year=2026&month=3');

    await expect(page.getByText('Energy Calendar', { exact: true })).toBeVisible();
    await expect(page.getByText('March 2026', { exact: true })).toBeVisible();
    await expect(page.getByText('Solar generated', { exact: true })).toBeVisible();
    await expect(page.getByText('Saved', { exact: true })).toBeVisible();
    await expect(page.getByTestId('energy-calendar-device-select')).toBeVisible();
    await expect(page.getByText('Sun', { exact: true })).toBeVisible();

    const firstWeekBoxes = await Promise.all(
      ['2026-03-01', '2026-03-02', '2026-03-03', '2026-03-04', '2026-03-05', '2026-03-06', '2026-03-07'].map(
        async (date) => page.getByTestId(`energy-calendar-day-${date}`).boundingBox()
      )
    );
    const firstWeekWidths = firstWeekBoxes.map((box) => Math.round(box?.width ?? 0));
    expect(new Set(firstWeekWidths).size).toBe(1);
    expect(firstWeekBoxes[6]?.x ?? 0).toBeGreaterThan(firstWeekBoxes[0]?.x ?? 0);
    expect(Math.round(firstWeekBoxes[0]?.y ?? 0)).toBe(Math.round(firstWeekBoxes[6]?.y ?? 0));
    const secondWeekFirstDay = await page.getByTestId('energy-calendar-day-2026-03-08').boundingBox();
    expect(Math.round(secondWeekFirstDay?.x ?? 0)).toBe(Math.round(firstWeekBoxes[0]?.x ?? 0));
    expect(secondWeekFirstDay?.y ?? 0).toBeGreaterThan(firstWeekBoxes[0]?.y ?? 0);

    const clickableDay = page.getByTestId('energy-calendar-day-2026-03-04');
    await expect(clickableDay).toHaveCSS('cursor', 'pointer');

    await page.getByTestId('energy-calendar-day-2026-03-04').click();
    await expect(page).toHaveURL(/\/energy\?/);
    await expect(page).toHaveURL(/date=2026-03-04/);
  });
});
