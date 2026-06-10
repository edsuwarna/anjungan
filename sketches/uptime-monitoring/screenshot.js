const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1440, height: 900 },
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();

  const htmlPath = 'file://' + path.resolve(__dirname, 'mockup.html');
  await page.goto(htmlPath, { waitUntil: 'networkidle0' });
  await page.waitForTimeout(1000);

  // Screenshot 1: List view
  await page.screenshot({ path: path.resolve(__dirname, 'uptime-monitor-list.png'), fullPage: true });
  console.log('Screenshot 1 saved: uptime-monitor-list.png');

  // Screenshot 2: Add Monitor form
  await page.click('.tab-btn:nth-child(2)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.resolve(__dirname, 'uptime-monitor-add.png'), fullPage: true });
  console.log('Screenshot 2 saved: uptime-monitor-add.png');

  // Screenshot 3: Monitor Detail
  await page.click('.tab-btn:nth-child(3)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.resolve(__dirname, 'uptime-monitor-detail.png'), fullPage: true });
  console.log('Screenshot 3 saved: uptime-monitor-detail.png');

  // Screenshot 4: Notification Targets
  await page.click('.tab-btn:nth-child(4)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.resolve(__dirname, 'uptime-notification-targets.png'), fullPage: true });
  console.log('Screenshot 4 saved: uptime-notification-targets.png');

  await browser.close();
  console.log('All screenshots done!');
})();
