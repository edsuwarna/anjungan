const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const DEVICE_SCALE = 2;
  const VIEWPORT = { width: 1440, height: 900 };
  const HTML_PATH = 'file://' + path.resolve(__dirname, 'trivy-scan-detail.html');

  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox', '--disable-setuid-sandbox'] });
  const context = await browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: DEVICE_SCALE,
    colorScheme: 'dark',
  });
  const page = await context.newPage();

  await page.goto(HTML_PATH, { waitUntil: 'networkidle0' });
  await page.waitForTimeout(2000);

  // Screenshot full page — it captures all sub-tabs stacked vertically is fine
  // But we want to show per tab. Let me capture each sub-tab.
  
  const tabBtns = await page.$$('.tab-btn');

  // Vulns tab (already active)
  await page.screenshot({ path: path.join(__dirname, 'detail-a-vulns.png'), fullPage: true });
  console.log('✓ Vulns tab');

  // Misconfig tab
  await tabBtns[1].click();
  await page.waitForTimeout(300);
  await page.screenshot({ path: path.join(__dirname, 'detail-b-misconfig.png'), fullPage: true });
  console.log('✓ Misconfig tab');

  // Secrets tab
  await tabBtns[2].click();
  await page.waitForTimeout(300);
  await page.screenshot({ path: path.join(__dirname, 'detail-c-secrets.png'), fullPage: true });
  console.log('✓ Secrets tab');

  // Raw JSON tab
  await tabBtns[3].click();
  await page.waitForTimeout(300);
  await page.screenshot({ path: path.join(__dirname, 'detail-d-rawjson.png'), fullPage: true });
  console.log('✓ Raw JSON tab');

  await browser.close();
  console.log('Done.');
})();
