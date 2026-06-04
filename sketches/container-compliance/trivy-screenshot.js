const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const DEVICE_SCALE = 2;
  const VIEWPORT = { width: 1440, height: 900 };
  const HTML_PATH = 'file://' + path.resolve(__dirname, 'trivy-mockup.html');

  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox', '--disable-setuid-sandbox'] });
  const context = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: DEVICE_SCALE,
    colorScheme: 'dark',
  });
  const page = await context.newPage();

  await page.goto(HTML_PATH, { waitUntil: 'networkidle0' });
  await page.waitForTimeout(2000);

  const tabBtns = await page.$$('.tab-btn');

  // TAB A: Dashboard
  await tabBtns[0].click();
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, 'trivy-a-dashboard.png'), fullPage: true });
  console.log('✓ Tab A');

  // TAB B: History
  await tabBtns[1].click();
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, 'trivy-b-history.png'), fullPage: true });
  console.log('✓ Tab B');

  // TAB C: Comparison
  await tabBtns[2].click();
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, 'trivy-c-comparison.png'), fullPage: true });
  console.log('✓ Tab C');

  await browser.close();
  console.log('Done.');
})();
