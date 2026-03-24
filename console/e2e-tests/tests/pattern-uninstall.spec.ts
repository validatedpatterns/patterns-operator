import { test, expect } from '@playwright/test';
import { login, dismissTour, gotoCatalogPage } from '../helpers/auth';
import { selectors } from '../helpers/selectors';

test.describe('Uninstall Pattern Page', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await dismissTour(page);

    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    // Show all tiers to find installed patterns
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
    await page.waitForSelector(selectors.uninstallPageTitle);
  });

  test('uninstall page shows pattern name in title', async ({ page }) => {
    const title = page.locator(selectors.uninstallPageTitle);
    await expect(title).toBeVisible();
    // Title format is "Uninstall Pattern: <name>"
    const text = await title.textContent();
    expect(text).toMatch(/Uninstall Pattern.*:.+/);
  });

  test('uninstall page shows pattern status card', async ({ page }) => {
    await expect(page.locator(selectors.patternStatusCard)).toBeVisible();
  });

  test('uninstall page shows warning and confirm button', async ({ page }) => {
    await expect(page.locator(selectors.uninstallWarning)).toBeVisible();
    await expect(page.locator(selectors.confirmUninstallButton)).toBeVisible();
  });

  test('uninstall page has cancel button that returns to catalog', async ({ page }) => {
    const cancelButton = page.locator(selectors.cancelButton);
    await expect(cancelButton).toBeVisible();
    await cancelButton.click();
    await expect(page).toHaveURL(/\/patterns$/);
  });
});
