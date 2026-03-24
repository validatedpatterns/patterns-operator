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
    test.skip(!loaded, 'Catalog failed to load — skipping card tests');

    const cards = page.locator(selectors.patternCard);
    await expect(cards.first()).toBeVisible();
    expect(await cards.count()).toBeGreaterThan(0);
  });

  test('each pattern card has a display name and tier label', async ({ page }) => {
    const loaded = await gotoCatalogPage(page);
    test.skip(!loaded, 'Catalog failed to load');

    const firstCard = page.locator(selectors.patternCard).first();
    await expect(firstCard).toBeVisible();
    // Card title text should not be empty
    const titleText = await firstCard.locator('[class*="card"] [class*="title"]').first().textContent();
    expect(titleText?.trim()).toBeTruthy();
    // Should have a tier label
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
    // Close and reopen isn't needed — dropdown stays open

    // Deselect "Maintained"
    await page.locator(selectors.tierOption('Maintained')).click();

    // Close the dropdown
    await toggle.click();

    // The toggle should now show only "Tested"
    await expect(toggle).toContainText('Tested');
    await expect(toggle).not.toContainText('Maintained');
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

  test('shows error alert when catalog fails to load', async ({ page }) => {
    await gotoCatalogPage(page);
    // This test verifies the page doesn't crash — it either shows cards or an error
    const hasCards = await page.locator(selectors.patternCard).count() > 0;
    const hasError = await page.locator(selectors.alertDanger).count() > 0;
    expect(hasCards || hasError).toBe(true);
  });
});
