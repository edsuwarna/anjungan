const puppeteer = require('puppeteer');
(async () => {
  const browser = await puppeteer.launch({
    headless: 'new',
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-gpu']
  });
  const page = await browser.newPage();

  // Increase default navigation timeout
  page.setDefaultNavigationTimeout(30000);

  await page.goto('file://' + __dirname + '/index.html', {
    waitUntil: 'networkidle0',
    timeout: 30000
  });
  await new Promise(r => setTimeout(r, 800));

  // 1) Global — set viewport to a reasonable height
  await page.evaluate(() => switchScreen('global'));
  await new Promise(r => setTimeout(r, 400));
  await page.setViewport({ width: 1440, height: 1200 });
  await new Promise(r => setTimeout(r, 200));
  await page.screenshot({ path: '01-global.png', fullPage: false });
  console.log('01-global.png done');

  // 2) L1 categories view
  await page.evaluate(() => {
    switchScreen('l1');
    document.getElementById('l1-category-detail').style.display = 'none';
  });
  await new Promise(r => setTimeout(r, 400));
  // adjust height to capture all category cards
  await page.setViewport({ width: 1440, height: 2000 });
  await new Promise(r => setTimeout(r, 200));
  await page.screenshot({ path: '02-l1-categories.png', fullPage: false });
  console.log('02-l1-categories.png done');

  // 3) L1 category detail (SSH)
  await page.evaluate(() => {
    switchScreen('l1');
    document.getElementById('l1-category-detail').style.display = 'block';
    openCategory('l1', 'ssh');
  });
  await new Promise(r => setTimeout(r, 400));
  await page.setViewport({ width: 1440, height: 1700 });
  await new Promise(r => setTimeout(r, 200));
  await page.screenshot({ path: '03-l1-category-detail.png', fullPage: false });
  console.log('03-l1-category-detail.png done');

  // 4) Lynis with category open
  await page.evaluate(() => {
    switchScreen('lynis');
  });
  await new Promise(r => setTimeout(r, 200));
  await page.setViewport({ width: 1440, height: 1200 });
  await new Promise(r => setTimeout(r, 200));
  await page.screenshot({ path: '04-lynis.png', fullPage: false });
  console.log('04-lynis.png done');

  // 5) Lynis category detail
  await page.evaluate(() => {
    switchScreen('lynis');
    document.getElementById('lynis-category-detail').style.display = 'block';
    openLynisCategory('auth');
  });
  await new Promise(r => setTimeout(r, 400));
  await page.setViewport({ width: 1440, height: 1200 });
  await new Promise(r => setTimeout(r, 200));
  await page.screenshot({ path: '05-lynis-detail.png', fullPage: false });
  console.log('05-lynis-detail.png done');

  await browser.close();
  console.log('all done');
})();
