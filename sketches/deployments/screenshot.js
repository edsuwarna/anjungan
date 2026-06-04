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

  // Variant A: Pipeline Cards - full page
  await page.click('#tab-a');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(outDir, 'variant-a-pipeline-cards.png'), fullPage: true });

  // Variant A: Click first card to expand detail
  const cards = await page.$$('.deploy-card');
  if (cards.length > 0) {
    await cards[0].click();
    await page.waitForTimeout(500);
  }
  await page.screenshot({ path: path.join(outDir, 'variant-a-detail-expanded.png'), fullPage: true });

  // Variant B: Timeline View
  await page.click('#tab-b');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(outDir, 'variant-b-timeline.png'), fullPage: true });

  console.log('Screenshots saved!');
  for (const f of fs.readdirSync(outDir)) {
    console.log(path.join(outDir, f));
  }

  await browser.close();
}

main().catch(err => { console.error(err); process.exit(1); });
