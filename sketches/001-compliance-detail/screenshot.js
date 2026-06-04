const puppeteer = require('puppeteer');
const path = require('path');

(async () => {
  const browser = await puppeteer.launch({
    headless: true,
    args: ['--no-sandbox', '--disable-setuid-sandbox'],
  });
  const page = await browser.newPage();
  await page.setViewport({ width: 1200, height: 900, deviceScaleFactor: 2 });

  const filePath = 'file://' + path.resolve(__dirname, 'variants.html');
  await page.goto(filePath, { waitUntil: 'networkidle0' });
  await new Promise(r => setTimeout(r, 1000));

  // Screenshot Variant A (default active)
  await page.screenshot({ path: path.join(__dirname, 'variant-a-overview.png'), fullPage: false });
  console.log('Captured: variant-a-overview.png');

  // Click SSH card in Variant A to open inline panel
  const sshCard = await page.evaluate(() => {
    // Find the SSH card in Variant A panel
    const cards = document.querySelectorAll('#panel-a .cat-card');
    if (cards.length > 0) cards[0].click(); // SSH is first
  });
  await new Promise(r => setTimeout(r, 500));
  await page.screenshot({ path: path.join(__dirname, 'variant-a-ssh-panel.png'), fullPage: true });
  console.log('Captured: variant-a-ssh-panel.png');

  // Switch to History tab in Variant A inline panel
  const histBtn = await page.evaluate(() => {
    const btn = document.querySelector('#subHistTab');
    if (btn) btn.click();
  });
  await new Promise(r => setTimeout(r, 300));
  await page.screenshot({ path: path.join(__dirname, 'variant-a-ssh-history.png'), fullPage: true });
  console.log('Captured: variant-a-ssh-history.png');

  // Switch to Variant B
  const tabB = await page.evaluate(() => {
    const tabs = document.querySelectorAll('.variant-tab');
    if (tabs.length > 1) tabs[1].click();
  });
  await new Promise(r => setTimeout(r, 500));
  await page.screenshot({ path: path.join(__dirname, 'variant-b-overview.png'), fullPage: false });
  console.log('Captured: variant-b-overview.png');

  // Switch to History tab in Variant B detail
  const vbHistBtn = await page.evaluate(() => {
    const btn = document.querySelector('#vbHistTab');
    if (btn) btn.click();
  });
  await new Promise(r => setTimeout(r, 300));
  await page.screenshot({ path: path.join(__dirname, 'variant-b-history.png'), fullPage: true });
  console.log('Captured: variant-b-history.png');

  await browser.close();
  console.log('Done!');
})();
