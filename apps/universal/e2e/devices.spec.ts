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

    await expect(page.getByText('Pulse Fleet', { exact: true })).toBeVisible();
    await expect(page.getByTestId('sidebar-search')).toHaveCount(0);
    await expect(page.getByText('Solar generation today', { exact: true })).toBeVisible();
    await expect(page.getByText('Storm Guard active for ~2h more')).toBeVisible();
    await expect(page.getByTestId(`device-card-${DPU_DEVICE_ID}`).getByText('DPU A 12 kWh', { exact: true })).toBeVisible();
    await expect(page.getByTestId(`device-card-${D2M_DEVICE_ID}`).getByText('Kitchen Delta 2 Max')).toBeVisible();
    await expect(page.getByText('Open Energy Dashboard', { exact: true })).toBeVisible();
    await page.getByTestId('header-weather-button').click();
    await expect(page).toHaveURL(/\/energy\?/);
    const energyUrl = new URL(page.url());
    expect(energyUrl.searchParams.get('device')).toBe('all');
    expect(energyUrl.searchParams.get('panel')).toBe('solar');

    await page.goto('/devices');
    await expect(page.getByTestId(`fleet-device-preview-${DPU_DEVICE_ID}`)).toBeVisible();
    await expect(page.getByTestId(`fleet-device-preview-${DPU_DEVICE_ID}`).getByText('24.6%')).toBeVisible();
    await expect(page.getByTestId(`fleet-device-preview-${D2M_DEVICE_ID}`)).toBeVisible();
    await expect(page.getByText('Current telemetry', { exact: true })).toHaveCount(0);
    await page.getByTestId('devices-analytics-panel').scrollIntoViewIfNeeded();
    await expect(page.getByTestId('home-energy-impact').getByText('Energy Impact', { exact: true })).toBeVisible();

    await page.getByTestId(`fleet-device-preview-${DPU_DEVICE_ID}`).click();
    await expect(page).toHaveURL(new RegExp(`/device/${DPU_DEVICE_ID}$`));

    await page.goto('/devices');
    await expect(page.getByRole('button', { name: /All Devices/i })).toBeVisible();

    await page.getByRole('button', { name: /All Devices/i }).click();
    await expect(page.getByTestId(`device-card-${DPU_DEVICE_ID}`)).toBeVisible();
  });

  test('loads partially visible analytics and defers offscreen card history', async ({ page }) => {
    const requestedHistoryPaths: string[] = [];
    page.on('request', (request) => {
      const path = new URL(request.url()).pathname;
      if (path.includes('/history')) {
        requestedHistoryPaths.push(path);
      }
    });

    await page.setViewportSize({ width: 1440, height: 900 });
    await page.goto('/devices');

    await expect(page.getByText('Pulse Fleet', { exact: true })).toBeVisible();
    await expect(page.getByText('Battery', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Open Energy Dashboard', { exact: true })).toBeVisible();

    const overflowX = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
    expect(overflowX).toBeLessThanOrEqual(1);

    const analyticsTop = await page.getByTestId('devices-analytics-panel').evaluate((element) =>
      element.getBoundingClientRect().top
    );
    expect(analyticsTop).toBeLessThan(900);
    await expect.poll(() =>
      requestedHistoryPaths.some((path) => /^\/api\/v1\/devices\/[^/]+\/history$/.test(path))
    ).toBe(true);
    expect(requestedHistoryPaths.filter((path) => /^\/api\/v1\/devices\/[^/]+\/history\/solar$/.test(path))).toHaveLength(0);
    expect(requestedHistoryPaths).toContain('/api/v1/history/solar/fleet');

    await page.getByTestId('devices-analytics-panel').scrollIntoViewIfNeeded();
    await expect(page.getByTestId('home-energy-impact')).toBeVisible();

    await page.getByTestId(`device-card-${DPU_DEVICE_ID}`).scrollIntoViewIfNeeded();
    await expect.poll(() =>
      requestedHistoryPaths.some((path) => /^\/api\/v1\/devices\/[^/]+\/history\/solar$/.test(path))
    ).toBe(true);
  });

  test('keeps the devices first fold stable across desktop tablet and phone widths', async ({ page }) => {
    const viewports = [
      { width: 1440, height: 900 },
      { width: 1024, height: 768 },
      { width: 390, height: 844 }
    ];

    for (const viewport of viewports) {
      await page.setViewportSize(viewport);
      await page.goto('/devices');

      await expect(page.getByText('Pulse Fleet', { exact: true })).toBeVisible();
      await expect(page.getByText('Solar generation today', { exact: true })).toBeVisible();
      await expect(page.getByTestId(`fleet-device-preview-${DPU_DEVICE_ID}`)).toBeVisible();
      await expect(page.getByTestId(viewport.width >= 1024 ? 'sidebar-search' : 'tab-search')).toHaveCount(0);

      const overflowX = await page.evaluate(() => document.documentElement.scrollWidth - document.documentElement.clientWidth);
      expect(overflowX).toBeLessThanOrEqual(1);
    }
  });

  test('loads detail by UUID route', async ({ page }) => {
    await page.goto(`/device/${DPU_DEVICE_ID}`);

    await expect(page).toHaveURL(new RegExp(`/device/${DPU_DEVICE_ID}$`));
    await expect(page.getByText('DPU A 12 kWh', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Battery reserve', { exact: true })).toBeVisible();
    await expect(page.getByText('Live telemetry', { exact: true })).toHaveCount(0);
    await expect(page.getByText('Battery Packs')).toBeVisible();
    await expect(page.getByText('Solar Inputs')).toBeVisible();
    await expect(page.getByText('Energy Impact', { exact: true })).toBeVisible();
    await expect(page.getByText('Device Solar Forecast', { exact: true })).toBeVisible();
    await expect(page.getByText('System Signals')).toBeVisible();
    await expect(page.getByText('Diagnostics', { exact: true })).toHaveCount(0);

    const impactBox = await page.getByTestId('device-energy-impact-panel').boundingBox();
    const forecastBox = await page.getByTestId('device-solar-forecast-panel').boundingBox();
    expect(impactBox).not.toBeNull();
    expect(forecastBox).not.toBeNull();
    expect(Math.abs((impactBox?.height ?? 0) - (forecastBox?.height ?? 0))).toBeLessThanOrEqual(2);

    await page.getByTestId('header-weather-button').click();
    await expect(page).toHaveURL(/\/energy\?/);
    const detailEnergyUrl = new URL(page.url());
    expect(detailEnergyUrl.searchParams.get('device')).toBe(DPU_DEVICE_ID);
    expect(detailEnergyUrl.searchParams.get('preset')).toBe('today');
    expect(detailEnergyUrl.searchParams.get('compare')).toBe('1');
    expect(detailEnergyUrl.searchParams.get('panel')).toBe('solar');

    await page.goto(`/device/${DPU_DEVICE_ID}`);
    await page.getByTestId('device-open-logs').click();
    await expect(page).toHaveURL(/\/logs\?/);
    const detailLogsUrl = new URL(page.url());
    expect(detailLogsUrl.searchParams.get('deviceId')).toBe(DPU_DEVICE_ID);
    expect(detailLogsUrl.searchParams.has('serial')).toBe(false);

    await page.goto(`/device/${DPU_DEVICE_ID}`);

    await page.getByTestId('sidebar-devices').click();

    await expect(page).toHaveURL(/\/devices$/);
    await expect(page.getByText('Pulse Fleet', { exact: true })).toBeVisible();
  });

  test('defers detail hardware insights until the hardware section approaches the viewport', async ({ page }) => {
    const requestedInsightPaths: string[] = [];
    page.on('request', (request) => {
      const path = new URL(request.url()).pathname;
      if (/^\/api\/v1\/devices\/[^/]+\/insights$/.test(path)) {
        requestedInsightPaths.push(path);
      }
    });

    await page.setViewportSize({ width: 390, height: 640 });
    await page.goto(`/device/${DPU_DEVICE_ID}`);

    await expect(page.getByText('Battery reserve', { exact: true })).toBeVisible();
    await expect(page.getByText('Power profile', { exact: true })).toBeAttached();
    await expect(page.getByTestId('device-hardware-section')).toBeAttached();
    await page.waitForTimeout(350);
    expect(requestedInsightPaths).toHaveLength(0);

    await page.getByTestId('device-hardware-section').scrollIntoViewIfNeeded();
    await expect(page.getByText('Battery Packs')).toBeVisible();
    await expect.poll(() => requestedInsightPaths.length).toBeGreaterThan(0);
  });

  test('does not resolve serial route aliases', async ({ page }) => {
    await page.goto(`/device/${D2M_SERIAL}`);

    await expect(page).toHaveURL(new RegExp(`/device/${D2M_SERIAL}$`));
    await expect(page.getByText('Device not found')).toBeVisible();
  });
});
