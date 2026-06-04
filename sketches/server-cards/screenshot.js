const { chromium } = require('playwright');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });

  const filePath = 'file://' + __dirname + '/index.html';
  await page.goto(filePath, { waitUntil: 'networkidle' });
  await page.waitForTimeout(500);

  // Screenshot Variant A (Compact Grid) - already active by default
  await page.screenshot({ path: __dirname + '/variant-a-compact.png', fullPage: false });
  console.log('Variant A captured');

  // Switch to Variant B (Detailed Cards)
  await page.click('#tab-b');
  await page.waitForTimeout(300);
  await page.screenshot({ path: __dirname + '/variant-b-detailed.png', fullPage: false });
  console.log('Variant B captured');

  // Also take a mobile viewport screenshot of variant B
  await page.setViewportSize({ width: 375, height: 812 });
  await page.waitForTimeout(300);
  await page.screenshot({ path: __dirname + '/variant-b-mobile.png', fullPage: true });
  console.log('Mobile variant captured');

  await browser.close();
})();
