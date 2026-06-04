const { chromium } = require('playwright');
const path = require('path');

const url = 'file://' + path.resolve(__dirname, 'variant-c-combo.html');
const outputDir = __dirname;

(async () => {
  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox'] });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 1050 },
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();

  await page.goto(url, { waitUntil: 'networkidle' });
  await page.waitForTimeout(500);

  await page.screenshot({
    path: path.join(outputDir, 'variant-c-combo.png'),
    fullPage: true,
  });
  console.log('Captured variant C');

  await browser.close();
  console.log('Done');
})();
