// Actual Page.goto, rather than Node request trust, proves Firefox fixture trust.
const receipt = { activation: 'unverified', category: 'validation', status: null, cleanup: 'not_started' };
let browser;
(async () => {
  const [origin, executable, policy] = process.argv.slice(2);
  const url = new URL(origin);
  if (process.argv.length !== 5 || origin.length > 2048 || executable?.length > 4096 || policy?.length > 4096 || url.protocol !== 'https:' || !['localhost', '127.0.0.1'].includes(url.hostname) ||
      !url.port || url.username || url.password || origin !== url.origin ||
      !executable?.startsWith('/') || !policy?.startsWith('/') ||
      process.env.PLAYWRIGHT_FIREFOX_POLICIES_JSON !== policy) throw new Error('validation');
  const { firefox } = require('@playwright/test');
  if (firefox.executablePath() !== executable) throw new Error('validation');
  receipt.category = 'launch';
  browser = await firefox.launch({ headless: true, timeout: 20000 });
  const context = await browser.newContext({ ignoreHTTPSErrors: false });
  const page = await context.newPage();
  receipt.category = 'navigation';
  const response = await page.goto(`${origin}/healthz`, { waitUntil: 'load', timeout: 20000 });
  if (!response) throw new Error('navigation');
  receipt.status = response.status();
  if (response.url() !== `${origin}/healthz`) throw new Error('redirect');
  if (receipt.status !== 200) throw new Error('http_status');
  receipt.activation = 'strict_browser_https';
  receipt.category = 'passed';
})().catch(error => {
  const message = String(error?.message ?? '');
  receipt.errorCode = ['SEC_ERROR_UNKNOWN_ISSUER', 'SEC_ERROR_UNTRUSTED_ISSUER', 'SEC_ERROR_EXPIRED_CERTIFICATE', 'SSL_ERROR_BAD_CERT_DOMAIN', 'NS_ERROR_NET_RESET'].find(code => message.includes(code)) ?? 'unclassified';
  if (message.includes('SEC_ERROR_') || message.includes('SSL_ERROR')) receipt.category = 'certificate';
  else if (message.includes('NS_ERROR_NET_RESET')) receipt.category = 'network_reset';
  else if (['validation', 'redirect', 'http_status', 'navigation'].includes(message)) receipt.category = message;
  process.exitCode = 1;
}).finally(async () => {
  if (browser) {
    try { await browser.close(); receipt.cleanup = 'closed'; }
    catch { receipt.cleanup = 'failed'; process.exitCode = 1; }
  }
  process.stdout.write(`${JSON.stringify(receipt)}\n`);
});
