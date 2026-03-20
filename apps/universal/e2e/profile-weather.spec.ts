import { expect, test } from '@playwright/test';
import { mockApiRoutes } from './mockApi';

test.describe('Profile weather web E2E', () => {
  test.beforeEach(async ({ page }) => {
    await page.addInitScript(() => {
      localStorage.setItem(
        'pulse-oidc-session-v1',
        JSON.stringify({
          state: {
            session: {
              issuerUrl: 'http://127.0.0.1:8084/realms/pulse',
              clientId: 'pulse-universal-app',
              accessToken: 'weather-test-access-token',
              refreshToken: 'weather-test-refresh-token',
              idToken: 'weather-test-id-token',
              tokenType: 'Bearer',
              expiresAtUnixMs: Date.now() + 60 * 60_000,
              updatedAtUnixMs: Date.now()
            }
          },
          version: 0
        })
      );
    });
    await mockApiRoutes(page);
  });

  test('renders compact weather and forecast cards on the profile page', async ({ page }) => {
    let yesterdayRequestCount = 0;
    page.on('request', (request) => {
      if (request.url().includes('/api/v1/weather/yesterday')) {
        yesterdayRequestCount += 1;
      }
    });

    await page.goto('/profile');

    await expect(page.getByText(/Solar 5\.2 kWh \+ est 7(?:\.0)? kWh today/)).toBeVisible();
    await expect(page.getByText('Current weather', { exact: true })).toBeVisible();
    await expect(page.getByText('7-day forecast', { exact: true })).toBeVisible();
    await expect(page.getByText('Yesterday verification', { exact: true })).toBeVisible();
    await expect(page.getByText('Open-Meteo data, CC BY 4.0.', { exact: false })).toBeVisible();
    await expect(page.getByText('12.9°C', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Rain', { exact: true }).first()).toBeVisible();
    await expect(page.getByText('Matched hours', { exact: true })).toHaveCount(0);
    expect(yesterdayRequestCount).toBe(0);

    await page.getByRole('button', { name: /Yesterday verification/i }).click();

    await expect.poll(() => yesterdayRequestCount).toBe(1);
    await expect(page.getByText('Matched hours', { exact: true })).toBeVisible();
    await expect(page.getByText(/ΔT /)).toHaveCount(24);
  });
});
