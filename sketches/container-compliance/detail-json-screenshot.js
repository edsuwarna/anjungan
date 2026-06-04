const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const DEVICE_SCALE = 2;
  const VIEWPORT = { width: 1440, height: 950 };
  const HTML_PATH = 'file://' + path.resolve(__dirname, 'detail-from-json.html');

  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox', '--disable-setuid-sandbox'] });
  const context = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: DEVICE_SCALE,
    colorScheme: 'dark',
  });
  const page = await context.newPage();

  await page.goto(HTML_PATH, { waitUntil: 'networkidle0' });
  await page.waitForTimeout(2000);

  // Show default view with all CVE cards collapsed
  await page.screenshot({ path: path.join(__dirname, 'detail-json-a-collapsed.png'), fullPage: true });
  console.log('✓ Collapsed view');

  // Click to expand the first CVE card (zero-day)
  const headers = await page.$$('.cve-header');
  if (headers.length > 0) {
    await headers[0].click();
    await page.waitForTimeout(300);
  }
  // Also expand the second CVE card (critical with fix)
  if (headers.length > 1) {
    await headers[1].click();
    await page.waitForTimeout(300);
  }
  
  await page.screenshot({ path: path.join(__dirname, 'detail-json-b-expanded.png'), fullPage: true });
  console.log('✓ Expanded view');

  await browser.close();
  console.log('Done.');
})();
