const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const DEVICE_SCALE = 2;
  const VIEWPORT = { width: 1440, height: 950 };
  const HTML_PATH = 'file://' + path.resolve(__dirname, 'scan-selector.html');

  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox', '--disable-setuid-sandbox'] });
  const context = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: DEVICE_SCALE,
    colorScheme: 'dark',
  });
  const page = await context.newPage();

  await page.goto(HTML_PATH, { waitUntil: 'networkidle0' });
  await page.waitForTimeout(2000);

  // Screenshot 1: Default view (Scan #005 selected)
  await page.screenshot({ path: path.join(__dirname, 'selector-a-scan005.png'), fullPage: true });
  console.log('✓ Scan #005');

  // Screenshot 2: Click Scan #002 to show switching
  await page.evaluate(() => {
    const pills = document.querySelectorAll('.scan-pill');
    // Find the scan #002 pill
    for (const p of pills) {
      if (p.dataset.scan === '002') {
        p.click();
        break;
      }
    }
  });
  await page.waitForTimeout(300);
  await page.screenshot({ path: path.join(__dirname, 'selector-b-scan002.png'), fullPage: true });
  console.log('✓ Scan #002');

  await browser.close();
  console.log('Done.');
})();
