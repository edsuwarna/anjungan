const { chromium } = require('playwright');
const path = require('path');

(async () => {
  const browser = await chromium.launch({ headless: true });
  const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });

  const filePath = 'file://' + path.resolve(__dirname, 'index.html');

  // === V1 Dashboard ===
  await page.goto(filePath, { waitUntil: 'networkidle' });
  await page.waitForTimeout(800);
  await page.screenshot({ path: path.join(__dirname, 'v1-dashboard.png'), fullPage: false });

  // === V1 Server Detail ===
  await page.click('.server-card:first-child');
  await page.waitForTimeout(600);
  await page.screenshot({ path: path.join(__dirname, 'v1-server-detail.png'), fullPage: false });

  // === V1: opened modal ===
  await page.click('.container-item:first-child');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, 'v1-container-modal.png'), fullPage: false });

  // === V1: click Exec button to show terminal ===
  const execBtn = await page.$('button:has(i.ri-code-s-slash-line)');
  if (execBtn) {
    await execBtn.click();
    await page.waitForTimeout(400);
  }
  await page.screenshot({ path: path.join(__dirname, 'v1-modal-exec.png'), fullPage: false });

  // Close modal via JS
  await page.evaluate(() => {
    document.getElementById('container-modal').classList.add('hidden');
    document.getElementById('modal-overlay').classList.add('hidden');
    document.getElementById('modal-content').classList.add('hidden');
    document.getElementById('exec-terminal')?.classList.add('hidden');
  });
  await page.waitForTimeout(300);

  // === V2 Split ===
  await page.click('#v2-tab');
  await page.waitForTimeout(600);
  await page.screenshot({ path: path.join(__dirname, 'v2-split-pane.png'), fullPage: false });

  // === V2: different server ===
  await page.click('.server-list-item:nth-child(3)');
  await page.waitForTimeout(500);
  await page.screenshot({ path: path.join(__dirname, 'v2-peladen-db.png'), fullPage: false });

  await browser.close();
  console.log('All screenshots OK');
})().catch(err => { console.error(err); process.exit(1); });
