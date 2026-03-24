import { test, expect } from '@playwright/test';
import { login, dismissTour } from '../helpers/auth';
import { selectors } from '../helpers/selectors';

test.describe('Pattern Catalog Page', () => {
  test.beforeEach(async ({ page }) => {
    await login(page);
    await dismissTour(page);
    await page.goto('/patterns');
    await page.waitForSelector(selectors.spinner, { state: 'detached', timeout: 30_000 });
  });

  test('displays the Pattern Catalog title', async ({ page }) => {
    await expect(page.locator(selectors.catalogPageTitle)).toBeVisible();
  });

  test('renders pattern cards from the catalog', async ({ page }) => {
    const cards = page.locator(selectors.patternCard);
    await expect(cards.first()).toBeVisible();
    const count = await cards.count();
    expect(count).toBeGreaterThan(0);
  });

  test('each pattern card has a display name and tier label', async ({ page }) => {
    const firstCard = page.locator(selectors.patternCard).first();
    await expect(firstCard.locator('.pf-v6-c-card__title')).not.toBeEmpty();
    await expect(firstCard.locator('.pf-v6-c-label').first()).toBeVisible();
  });

  test('tier filter defaults to Maintained', async ({ page }) => {
    const toggle = page.locator(`${selectors.tierFilterToggle} >> xpath=..`).locator('.pf-v6-c-menu-toggle');
    await expect(toggle).toContainText('Maintained');
  });

  test('tier filter can select and deselect tiers', async ({ page }) => {
    // Open the tier filter
    const toggle = page.locator(`${selectors.tierFilterToggle} >> xpath=..`).locator('.pf-v6-c-menu-toggle');
    await toggle.click();

    // Select "Tested" tier
    await page.locator(selectors.tierOption('Tested')).click();
    await expect(toggle).toContainText('Tested');

    // Deselect "Maintained"
    await page.locator(selectors.tierOption('Maintained')).click();

    // Close the dropdown
    await toggle.click();

    // All visible cards should have "tested" tier label
    const cards = page.locator(selectors.patternCard);
    const count = await cards.count();
    for (let i = 0; i < count; i++) {
      const tierLabel = cards.nth(i).locator('.pf-v6-c-label').first();
      await expect(tierLabel).toHaveText('tested');
    }
  });

  test('pattern cards show cloud provider labels when available', async ({ page }) => {
    const cloudLabels = page.locator(selectors.cloudLabel);
    const count = await cloudLabels.count();
    if (count > 0) {
      const text = await cloudLabels.first().textContent();
      expect(['AWS', 'GCP', 'Azure']).toContain(text?.trim());
    }
  });

  test('non-installed patterns show Install button', async ({ page }) => {
    const installButtons = page.locator(selectors.patternCard).locator(selectors.installButton);
    const count = await installButtons.count();
    if (count > 0) {
      await expect(installButtons.first()).toBeVisible();
    }
  });

  test('pattern cards show Docs and Repo links', async ({ page }) => {
    const docsLinks = page.locator(selectors.patternCard).locator(selectors.docsLink);
    const count = await docsLinks.count();
    if (count > 0) {
      const href = await docsLinks.first().getAttribute('href');
      expect(href).toBeTruthy();
      expect(href).toMatch(/^https?:\/\//);
    }
  });

  test('no error alerts are shown', async ({ page }) => {
    await expect(page.locator(selectors.alertDanger)).not.toBeVisible();
  });
});
