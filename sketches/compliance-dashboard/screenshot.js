const { chromium } = require('playwright');
const path = require('path');

const url = 'file://' + path.resolve(__dirname, 'two-takes.html');
const outputDir = __dirname;

(async () => {
  const browser = await chromium.launch({ headless: true, args: ['--no-sandbox'] });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 900 },
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();

  await page.goto(url, { waitUntil: 'networkidle' });
  await page.waitForTimeout(500);

  // Screenshot Variant A: Score Cards
  await page.screenshot({
    path: path.join(outputDir, 'variant-a-score-cards.png'),
    fullPage: true,
  });
  console.log('Captured variant A');

  // Switch to Variant B: Compact Table
  await page.click('#tab-b');
  await page.waitForTimeout(400);
  await page.screenshot({
    path: path.join(outputDir, 'variant-b-compact-table.png'),
    fullPage: true,
  });
  console.log('Captured variant B');

  await browser.close();
  console.log('Done');
})();
