const { chromium } = require('playwright');
const path = require('path');

const MOCKUP_FILE = 'index.html';
const VIEWPORT = { width: 1440, height: 900 };
const OUTPUT_DIR = __dirname;

const states = [
  { name: '01-full-view', action: async (page) => {
    await page.waitForTimeout(600);
  }},

  { name: '02-filtered-critical', action: async (page) => {
    // Click "🔴 Critical" status filter
    const critBtn = page.locator('button[data-filter="crit"]');
    await critBtn.click();
    await page.waitForTimeout(300);
  }},

  { name: '03-findings-collapsed', action: async (page) => {
    // Reset filters - click All first to reset
    const allBtn = page.locator('button[data-filter="sall"]');
    await allBtn.click();
    await page.waitForTimeout(200);
    // Collapse the top findings section
    const findingsHeader = page.locator('.collapsible-header').first();
    await findingsHeader.click();
    await page.waitForTimeout(300);
  }},
];

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: VIEWPORT });

  const filePath = 'file://' + path.resolve(__dirname, MOCKUP_FILE);
  await page.goto(filePath, { waitUntil: 'networkidle' });

  for (const state of states) {
    try {
      await state.action(page);
      const outPath = path.join(OUTPUT_DIR, `${state.name}.png`);
      await page.screenshot({ path: outPath, fullPage: false });
      console.log(`✅ ${state.name}.png`);
    } catch (err) {
      console.error(`❌ ${state.name}: ${err.message}`);
    }
  }

  await browser.close();
  console.log(`\n✔ Done — ${states.length} screenshots in ${OUTPUT_DIR}`);
})().catch(err => { console.error(err); process.exit(1); });
