const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({
    viewport: { width: 1280, height: 900 },
    deviceScaleFactor: 2,
  });
  const page = await context.newPage();

  const htmlPath = 'file://' + path.resolve(__dirname, 'mockup.html');
  await page.goto(htmlPath, { waitUntil: 'networkidle0' });
  await page.waitForTimeout(1000);

  // 1 — Project list (admin)
  await page.click('.tab-btn:nth-child(1)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: 'projects-admin-list.png', fullPage: true });

  // 2 — Project switcher dropdown
  await page.click('.tab-btn:nth-child(2)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: 'projects-switcher.png', fullPage: true });

  // 3 — Project overview dashboard
  await page.click('.tab-btn:nth-child(3)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: 'projects-overview.png', fullPage: true });

  // 4 — Project-scoped servers
  await page.click('.tab-btn:nth-child(4)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: 'projects-servers.png', fullPage: true });

  // 5 — Project settings / members
  await page.click('.tab-btn:nth-child(5)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: 'projects-settings.png', fullPage: true });

  await browser.close();
  console.log('All screenshots saved');
})();
