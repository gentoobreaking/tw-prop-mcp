const puppeteer = require('puppeteer');

(async () => {
  const browser = await puppeteer.launch({
    headless: 'new',
    args: ['--no-sandbox', '--disable-setuid-sandbox', '--disable-web-security'],
  });
  const page = await browser.newPage();
  
  const errors = [];
  const consoleMessages = [];
  
  page.on('console', (msg) => {
    const text = msg.text();
    if (text && !text.includes('Download the React')) {
      consoleMessages.push(`${msg.type()}: ${text}`);
    }
  });
  
  page.on('pageerror', (err) => {
    errors.push(`PageError: ${err.message}`);
  });
  
  page.on('requestfailed', (req) => {
    errors.push(`RequestFailed: ${req.url()} - ${req.failure()?.errorText}`);
  });
  
  console.log('Navigating to http://localhost:80...');
  await page.goto('http://localhost:80/', { waitUntil: 'networkidle0', timeout: 30000 });
  
  // Wait for the app to render
  await page.waitForTimeout(3000);
  
  console.log('Page title:', await page.title());
  
  // Check for the search input
  const searchInput = await page.$('input[placeholder*="搜尋"]');
  console.log('Search input found:', !!searchInput);
  
  // Type and search "臺北市 中正區"
  if (searchInput) {
    await searchInput.type('臺北市 中正區');
    console.log('Typed search query');
    await page.waitForTimeout(500);
    
    const submitButton = await page.$('button[aria-label="搜尋"]');
    if (submitButton) {
      await submitButton.click();
      console.log('Clicked search');
    }
    
    // Wait for results
    await page.waitForTimeout(5000);
    
    // Check for search error or results
    const errorEl = await page.$('.search-error');
    const errorText = errorEl ? await page.evaluate(el => el.textContent, errorEl) : null;
    console.log('Search error:', errorText);
    
    const resultsEl = await page.$('.search-results-list');
    const resultsText = resultsEl ? await page.evaluate(el => el.textContent?.substring(0, 200), resultsEl) : null;
    console.log('Search results:', resultsText?.substring(0, 200));
    
    // Check map
    const mapEl = await page.$('.map-container, #map, .leaflet-container');
    console.log('Map element found:', !!mapEl);
  }
  
  console.log('\n=== Console messages ===');
  consoleMessages.forEach(msg => console.log(msg));
  
  console.log('\n=== Errors ===');
  errors.forEach(err => console.log(err));
  
  await browser.close();
  process.exit(errors.length > 0 ? 1 : 0);
})().catch(e => { console.error(e); process.exit(1); });
