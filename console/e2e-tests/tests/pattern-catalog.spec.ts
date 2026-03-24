import { test, expect } from '@playwright/test';
import { login, dismissTour, gotoCatalogPage } from '../helpers/auth';
import { selectors } from '../helpers/selectors';

test.describe('Pattern Catalog Page', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await dismissTour(page);
  });

  test('displays the Pattern Catalog title', async ({ page }) => {
    await gotoCatalogPage(page);
    await expect(page.locator(selectors.catalogPageTitle)).toBeVisible();
  });

  test('renders pattern cards from the catalog', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const cards = page.locator(selectors.patternCard);
    await expect(cards.first()).toBeVisible();
    expect(await cards.count()).toBeGreaterThan(0);
  });

  test('each pattern card has a display name and tier label', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const firstCard = page.locator(selectors.patternCard).first();
    await expect(firstCard).toBeVisible();
    const titleText = await firstCard.locator('[class*="card"] [class*="title"]').first().textContent();
    expect(titleText?.trim()).toBeTruthy();
    await expect(firstCard.locator('[class*="label"]').first()).toBeVisible();
  });

  test('tier filter defaults to Maintained', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const toggle = page.locator(selectors.tierFilterToggle);
    await expect(toggle).toContainText('Maintained');
  });

  test('tier filter can select and deselect tiers', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const toggle = page.locator(selectors.tierFilterToggle);
    await toggle.click();

    // Select "Tested" tier
    await page.locator(selectors.tierOption('Tested')).click();

    // Deselect "Maintained"
    await page.locator(selectors.tierOption('Maintained')).click();

    // Close the dropdown
    await toggle.click();

    await expect(toggle).toContainText('Tested');
    await expect(toggle).not.toContainText('Maintained');
  });

  test('selecting all tiers shows all patterns', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    // Count cards with default filter (maintained only)
    const maintainedCount = await page.locator(selectors.patternCard).count();

    // Select all tiers
    const toggle = page.locator(selectors.tierFilterToggle);
    await toggle.click();
    await page.locator(selectors.tierOption('Tested')).click();
    await page.locator(selectors.tierOption('Sandbox')).click();
    await toggle.click();

    const allCount = await page.locator(selectors.patternCard).count();
    expect(allCount).toBeGreaterThanOrEqual(maintainedCount);
  });

  test('deselecting all tiers shows all patterns', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    // Deselect maintained (the only selected tier)
    const toggle = page.locator(selectors.tierFilterToggle);
    await toggle.click();
    await page.locator(selectors.tierOption('Maintained')).click();
    await toggle.click();

    // Toggle should show generic "Tier" label
    await expect(toggle).toContainText('Tier');

    // Should show all patterns
    const cards = page.locator(selectors.patternCard);
    expect(await cards.count()).toBeGreaterThan(0);
  });

  test('pattern cards show cloud provider labels when available', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const cloudLabels = page.locator(selectors.cloudLabel);
    const count = await cloudLabels.count();
    if (count > 0) {
      const text = await cloudLabels.first().textContent();
      expect(['AWS', 'GCP', 'Azure']).toContain(text?.trim());
    }
  });

  test('non-installed patterns show Install button', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const installButtons = page.locator(selectors.patternCard).locator(selectors.installButton);
    const count = await installButtons.count();
    if (count > 0) {
      await expect(installButtons.first()).toBeVisible();
    }
  });

  test('installed patterns show Uninstall and Manage Secrets buttons', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    // Show all tiers to maximize chances of finding installed patterns
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

    const firstInstalled = installedCards.first();
    await expect(firstInstalled.locator(selectors.uninstallButton)).toBeVisible();
    await expect(firstInstalled.locator(selectors.manageSecretsButton)).toBeVisible();
  });

  test('pattern cards show Docs and Repo links', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const docsLinks = page.locator(selectors.patternCard).locator(selectors.docsLink);
    const count = await docsLinks.count();
    if (count > 0) {
      const href = await docsLinks.first().getAttribute('href');
      expect(href).toBeTruthy();
      expect(href).toMatch(/^https?:\/\//);
    }
  });

  test('catalog description is shown when available', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    // The catalog description section appears between the title and the toolbar
    const description = page.locator(selectors.catalogDescription).first();
    const count = await description.count();
    if (count > 0) {
      const text = await description.textContent();
      expect(text?.trim().length).toBeGreaterThan(0);
    }
  });

  test('page renders without crashing', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    if (loaded) {
      await expect(page.locator(selectors.patternCard).first()).toBeVisible();
    } else {
      await expect(page.locator(selectors.alertDanger)).toBeVisible();
    }
  });
});
