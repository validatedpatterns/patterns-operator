import { test, expect } from '@playwright/test';
import { login, dismissTour } from '../helpers/auth';
import { selectors } from '../helpers/selectors';

test.describe('Pattern Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await dismissTour(page);
  });

  test('Install button navigates to install page', async ({ page }) => {
    await page.goto('/patterns');
    await page.waitForSelector(selectors.spinner, { state: 'detached', timeout: 30_000 });

    const installButton = page.locator(selectors.patternCard).locator(selectors.installButton).first();
    const count = await installButton.count();
    test.skip(count === 0, 'No installable patterns available');

    await installButton.click();
    await expect(page).toHaveURL(/\/patterns\/install\/.+/);
    await expect(page.locator(selectors.installPageTitle)).toBeVisible();
  });

  test('Cancel on install page returns to catalog', async ({ page }) => {
    await page.goto('/patterns');
    await page.waitForSelector(selectors.spinner, { state: 'detached', timeout: 30_000 });

    const installButton = page.locator(selectors.patternCard).locator(selectors.installButton).first();
    const count = await installButton.count();
    test.skip(count === 0, 'No installable patterns available');

    await installButton.click();
    await page.waitForSelector(selectors.installPageTitle);
    await page.locator(selectors.cancelButton).click();
    await expect(page).toHaveURL(/\/patterns$/);
  });
});
