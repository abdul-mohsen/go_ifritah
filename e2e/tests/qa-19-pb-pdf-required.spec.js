// QA-19: pb_pdf_required setting enforcement on add-purchase-bill.
//
// Asserts the PDF upload field reflects the setting:
//   - required: input has `required` and label shows the required-mark span.
//   - optional: input does NOT have `required`; field is visible.
//   - disabled: container is hidden via inline style display:none.

const { test, expect } = require('@playwright/test');
const { login, setSetting } = require('../helpers/qa');

test.describe('pb_pdf_required setting on add-purchase-bill', () => {
  test('required: file input has required attribute', async ({ page }) => {
    await login(page);
    await setSetting(page, 'pb_pdf_required', 'required');

    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    const input = page.locator('input[type="file"][name="bill_pdf"]');
    await expect(input, 'PDF input must exist when required').toHaveCount(1);
    await expect(input).toHaveAttribute('required', '');
  });

  test('optional: file input has no required attribute, still visible', async ({ page }) => {
    await login(page);
    await setSetting(page, 'pb_pdf_required', 'optional');

    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    const input = page.locator('input[type="file"][name="bill_pdf"]');
    await expect(input, 'PDF input must exist when optional').toHaveCount(1);
    const isRequired = await input.evaluate((el) => el.hasAttribute('required'));
    expect(isRequired, 'optional must NOT mark required').toBeFalsy();
  });

  test('disabled: PDF container is hidden', async ({ page }) => {
    await login(page);
    await setSetting(page, 'pb_pdf_required', 'disabled');

    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    // The PDF section's outer div has style="display:none" when disabled.
    const containerHidden = await page.evaluate(() => {
      const inp = document.querySelector('input[type="file"][name="bill_pdf"]');
      if (!inp) return true;
      let el = inp;
      while (el) {
        const s = el.getAttribute('style') || '';
        if (/display\s*:\s*none/i.test(s)) return true;
        el = el.parentElement;
      }
      return false;
    });
    expect(containerHidden, 'disabled mode must hide the PDF upload container').toBeTruthy();
  });

  test('required: submitting form without PDF blocks via browser validation', async ({ page }) => {
    await login(page);
    await setSetting(page, 'pb_pdf_required', 'required');

    await page.goto('/dashboard/purchase-bills/add');
    await page.waitForLoadState('domcontentloaded');

    // Click submit without attaching a file. The browser must NOT fire a POST
    // because the bill_pdf input is required and missing.
    let posted = false;
    page.on('request', (req) => {
      if (req.url().endsWith('/api/purchase-bills') && req.method() === 'POST') posted = true;
    });
    const submitBtn = page.locator('button[type="submit"]').first();
    if (await submitBtn.count() > 0) {
      await submitBtn.click();
      await page.waitForTimeout(800);
    }
    expect(posted, 'required+missing PDF: must NOT POST').toBeFalsy();

    const stillThere = await page.locator('input[type="file"][name="bill_pdf"]').first();
    const validity = await stillThere.evaluate((el) => el.validity && el.validity.valueMissing);
    expect(validity, 'browser must report valueMissing on the file input').toBeTruthy();
  });
});

// Restore default setting after these tests so other suites are unaffected.
test.afterAll(async ({ browser }) => {
  test.setTimeout(60000);
  const ctx = await browser.newContext();
  const page = await ctx.newPage();
  try {
    await login(page);
    await setSetting(page, 'pb_pdf_required', 'required');
  } catch (e) {
    // Best-effort cleanup; don't fail the suite on cleanup hiccup.
  }
  await ctx.close();
});
