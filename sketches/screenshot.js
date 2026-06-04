const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 900 },
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();

  const htmlPath = 'file://' + path.resolve(__dirname, 'cis-docker-mockup.html');
  await page.goto(htmlPath, { waitUntil: 'networkidle0' });
  await page.waitForTimeout(1000);

  // Phase 1 screenshot
  await page.click('.tab-btn:first-child');
  await page.waitForTimeout(500);
  await page.screenshot({ path: 'phase1-cis-docker.png', fullPage: true });

  // Phase 2 screenshot
  await page.click('.tab-btn:last-child');
  await page.waitForTimeout(500);
  await page.screenshot({ path: 'phase2-container-scan.png', fullPage: true });

  await browser.close();
  console.log('Screenshots saved');
})();
