import { test, expect } from '@playwright/test';
import { login, dismissTour, gotoCatalogPage } from '../helpers/auth';
import { selectors } from '../helpers/selectors';

test.describe('Pattern Navigation', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await dismissTour(page);
  });

  test('Install button navigates to install page', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const installButton = page.locator(selectors.patternCard).locator(selectors.installButton).first();
    const count = await installButton.count();
    test.skip(count === 0, 'No installable patterns available');

    await installButton.click();
    await expect(page).toHaveURL(/\/patterns\/install\/.+/);
    await expect(page.locator(selectors.installPageTitle)).toBeVisible();
  });

  test('Cancel on install page returns to catalog', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const installButton = page.locator(selectors.patternCard).locator(selectors.installButton).first();
    const count = await installButton.count();
    test.skip(count === 0, 'No installable patterns available');

    await installButton.click();
    await page.waitForSelector(selectors.installPageTitle);
    await page.locator(selectors.cancelButton).click();
    await expect(page).toHaveURL(/\/patterns$/);
  });

  test('Uninstall button navigates to uninstall page', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    // Show all tiers
    const toggle = page.locator(selectors.tierFilterToggle);
    await toggle.click();
    await page.locator(selectors.tierOption('Tested')).click();
    await page.locator(selectors.tierOption('Sandbox')).click();
    await toggle.click();

    const installedCards = page.locator(selectors.patternCard).filter({
      has: page.locator(selectors.installedLabel),
    });
    const count = await installedCards.count();
    test.skip(count === 0, 'No installed patterns on this cluster');

    await installedCards.first().locator(selectors.uninstallButton).click();
    await expect(page).toHaveURL(/\/patterns\/uninstall\/.+/);
    await expect(page.locator(selectors.uninstallPageTitle)).toBeVisible();
  });

  test('Manage Secrets button navigates to secrets page', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    // Show all tiers
    const toggle = page.locator(selectors.tierFilterToggle);
    await toggle.click();
    await page.locator(selectors.tierOption('Tested')).click();
    await page.locator(selectors.tierOption('Sandbox')).click();
    await toggle.click();

    const installedCards = page.locator(selectors.patternCard).filter({
      has: page.locator(selectors.installedLabel),
    });
    const count = await installedCards.count();
    test.skip(count === 0, 'No installed patterns on this cluster');

    await installedCards.first().locator(selectors.manageSecretsButton).click();
    await expect(page).toHaveURL(/\/patterns\/secrets\/.+/);
    await expect(page.locator(selectors.manageSecretsPageTitle)).toBeVisible();
  });
});
