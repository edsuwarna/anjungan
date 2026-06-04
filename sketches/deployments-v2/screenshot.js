const { chromium } = require('playwright');
const path = require('path');
const fs = require('fs');

const DEVICE_SCALE = 2;
const VIEWPORT = { width: 1440, height: 900 };

async function main() {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { ...VIEWPORT, deviceScaleFactor: DEVICE_SCALE } });

  const htmlPath = 'file://' + path.resolve(__dirname, 'mockup.html');
  await page.goto(htmlPath, { waitUntil: 'networkidle0' });
  await page.waitForTimeout(2000);

  const outDir = path.resolve(__dirname, 'screenshots');
  fs.mkdirSync(outDir, { recursive: true });

  // Screenshot 1: Production tab (default)
  await page.screenshot({ path: path.join(outDir, '01-production-tab.png'), fullPage: true });

  // Screenshot 2: Switch to Staging tab
  const stagingTab = await page.$('.env-tab[data-env="staging"]');
  if (stagingTab) await stagingTab.click();
  await page.waitForTimeout(400);
  await page.screenshot({ path: path.join(outDir, '02-staging-tab.png'), fullPage: true });

  // Screenshot 3: Show Manage Environments panel
  const manageBtn = await page.$('button:has-text("Manage Environments")');
  if (manageBtn) await manageBtn.click();
  await page.waitForTimeout(400);
  await page.screenshot({ path: path.join(outDir, '03-manage-environments.png'), fullPage: true });

  // Screenshot 4: Show New Deployment modal
  const newBtn = await page.$('button:has-text("New Deployment")');
  if (newBtn) await newBtn.click();
  await page.waitForTimeout(400);
  await page.screenshot({ path: path.join(outDir, '04-new-deployment-modal.png'), fullPage: true });

  console.log('Screenshots saved!');
  for (const f of fs.readdirSync(outDir)) {
    console.log(path.join(outDir, f));
  }

  await browser.close();
}

main().catch(err => { console.error(err); process.exit(1); });
