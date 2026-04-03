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

  test('points profile users to the dedicated Energy solar workspace', async ({ page }) => {
    await page.goto('/profile');

    await expect(page.getByText(/Solar 5\.2 kWh \+ 1\.8 kWh est/)).toBeVisible();
    await expect(page.getByText('Solar moved to Energy', { exact: true })).toBeVisible();
    await expect(
      page.getByText(
        'Solar forecast, weather verification, and the larger Energy Impact pane now live under Energy so site and device deep links share one consistent flow.',
        { exact: true }
      )
    ).toBeVisible();
    await expect(page.getByRole('button', { name: /Open Solar/i })).toBeVisible();
  });
});
