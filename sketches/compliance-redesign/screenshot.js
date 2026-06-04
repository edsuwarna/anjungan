const { chromium } = require('playwright');
const path = require('path');

const HTML_PATH = 'file://' + path.resolve(__dirname, 'mockup.html');
const DEVICE_SCALE = 2;

async function run() {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1200, height: 900 },
    deviceScaleFactor: DEVICE_SCALE,
  });
  const page = await context.newPage();
  await page.goto(HTML_PATH, { waitUntil: 'networkidle0' });
  await page.waitForTimeout(1000);

  // Screenshot 1: Compliance Overview
  await page.click('.screen-tab:nth-child(1)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, '01-compliance-overview.png'), fullPage: true });
  console.log('01-compliance-overview.png captured');

  // Screenshot 2: Server Detail - Overview Tab
  await page.click('.screen-tab:nth-child(2)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, '02-server-detail.png'), fullPage: true });
  console.log('02-server-detail.png captured');

  // Screenshot 3: Server - Compliance Tab (CIS L1)
  await page.click('.screen-tab:nth-child(3)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, '03-server-compliance.png'), fullPage: true });
  console.log('03-server-compliance.png captured');

  // Screenshot 4: Server - Lynis Tab
  await page.click('.screen-tab:nth-child(4)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, '04-server-lynis.png'), fullPage: true });
  console.log('04-server-lynis.png captured');

  await browser.close();
  console.log('All screenshots captured successfully!');
}

run().catch(err => {
  console.error('Screenshot failed:', err);
  process.exit(1);
});
