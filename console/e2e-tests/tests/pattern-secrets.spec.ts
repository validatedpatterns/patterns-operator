import { test, expect } from '@playwright/test';
import { login, dismissTour, gotoCatalogPage } from '../helpers/auth';
import { selectors } from '../helpers/selectors';

test.describe('Manage Secrets Page', () => {
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

    await installedCards.first().locator(selectors.manageSecretsButton).click();
    // Wait for either the secrets form or a "no secrets" info alert
    await Promise.race([
      page.waitForSelector(selectors.manageSecretsPageTitle, { timeout: 15_000 }),
      page.waitForSelector('.pf-v6-c-alert.pf-m-info', { timeout: 15_000 }),
    ]);
  });

  test('manage secrets page shows title with pattern name', async ({ page }) => {
    const title = page.locator(selectors.manageSecretsPageTitle);
    const noSecrets = page.locator('.pf-v6-c-alert.pf-m-info:has-text("No secrets configured")');

    if (await noSecrets.count() > 0) {
      // Pattern has no secret template — page shows info alert
      await expect(noSecrets).toBeVisible();
      return;
    }

    await expect(title).toBeVisible();
    const text = await title.textContent();
    expect(text).toMatch(/Manage Secrets for .+/);
  });

  test('manage secrets page shows secret configuration info', async ({ page }) => {
    const noSecrets = page.locator('.pf-v6-c-alert.pf-m-info:has-text("No secrets configured")');
    test.skip(await noSecrets.count() > 0, 'Pattern has no secret template');

    await expect(page.locator(selectors.secretConfigAlert)).toBeVisible();
  });

  test('manage secrets page shows expandable secret sections', async ({ page }) => {
    const noSecrets = page.locator('.pf-v6-c-alert.pf-m-info:has-text("No secrets configured")');
    test.skip(await noSecrets.count() > 0, 'Pattern has no secret template');

    const sections = page.locator(selectors.secretSection);
    expect(await sections.count()).toBeGreaterThan(0);

    // First section should be expanded by default
    const firstSection = sections.first();
    const fields = firstSection.locator(selectors.secretField);
    expect(await fields.count()).toBeGreaterThan(0);
  });

  test('manage secrets page has inject and back buttons', async ({ page }) => {
    const noSecrets = page.locator('.pf-v6-c-alert.pf-m-info:has-text("No secrets configured")');
    test.skip(await noSecrets.count() > 0, 'Pattern has no secret template');

    await expect(page.locator(selectors.injectSecretsButton)).toBeVisible();
    await expect(page.locator(selectors.backToCatalogLink)).toBeVisible();
  });

  test('back to catalog button returns to catalog page', async ({ page }) => {
    const backButton = page.locator(selectors.backToCatalogLink);
    await expect(backButton).toBeVisible();
    await backButton.click();
    await expect(page).toHaveURL(/\/patterns$/);
  });
});
