const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const DEVICE_SCALE = 2; // retina quality
  const VIEWPORT = { width: 1440, height: 900 };
  const HTML_PATH = 'file://' + path.resolve(__dirname, 'mockup.html');

  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox', '--disable-setuid-sandbox'] });
  const context = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: DEVICE_SCALE,
    colorScheme: 'dark',
  });
  const page = await context.newPage();

  await page.goto(HTML_PATH, { waitUntil: 'networkidle0' });
  // Extra wait for fonts/tailwind to settle
  await page.waitForTimeout(2000);

  // --- TAB A: Compliance Dashboard ---
  const tabBtns = await page.$$('.tab-btn');
  await tabBtns[0].click();
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, 'tab-a-dashboard.png'), fullPage: true });
  console.log('✓ Tab A captured: tab-a-dashboard.png');

  // --- TAB B: Docker Detail ---
  await tabBtns[1].click();
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, 'tab-b-docker-detail.png'), fullPage: true });
  console.log('✓ Tab B captured: tab-b-docker-detail.png');

  // --- TAB C: Trivy Vulnerability ---
  await tabBtns[2].click();
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, 'tab-c-trivy.png'), fullPage: true });
  console.log('✓ Tab C captured: tab-c-trivy.png');

  await browser.close();
  console.log('All screenshots done.');
})();
