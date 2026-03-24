import { type Page } from '@playwright/test';

export async function login(page: Page): Promise<void> {
  await page.goto('/');

  const authDisabled = await page.evaluate(() => {
    return (window as any).SERVER_FLAGS?.authDisabled;
  });

  if (authDisabled) {
    return;
  }

  const username = process.env.BRIDGE_KUBEADMIN_USERNAME || 'kubeadmin';
  const password = process.env.BRIDGE_KUBEADMIN_PASSWORD;

  if (!password) {
    throw new Error('BRIDGE_KUBEADMIN_PASSWORD must be set when auth is enabled');
  }

  await page.fill('#inputUsername', username);
  await page.fill('#inputPassword', password);
  await page.click('button[type=submit]');
  await page.waitForSelector('[data-test="username"]');
}

export async function dismissTour(page: Page): Promise<void> {
  const skipButton = page.locator('[data-test="tour-step-footer-secondary"]', {
    hasText: 'Skip tour',
  });
  try {
    await skipButton.click({ timeout: 5000 });
  } catch {
    // Tour may not appear
  }
}

/**
 * Navigate to /patterns and wait for loading to complete (either success or error).
 * Returns true if patterns loaded successfully, false if an error occurred.
 */
export async function gotoCatalogPage(page: Page): Promise<boolean> {
  await page.goto('/patterns');
  // Wait for either spinner to disappear or error alert to appear
  await Promise.race([
    page.waitForSelector('.pf-v6-c-spinner', { state: 'detached', timeout: 30_000 }).catch(() => {}),
    page.waitForSelector('.pf-v6-c-alert.pf-m-danger', { state: 'visible', timeout: 30_000 }).catch(() => {}),
  ]);
  // Give a moment for the DOM to settle
  await page.waitForTimeout(500);
  const hasError = await page.locator('.pf-v6-c-alert.pf-m-danger').count() > 0;
  return !hasError;
}
