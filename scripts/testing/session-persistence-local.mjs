import {chromium, expect} from '../../apps/player/e2e/node_modules/@playwright/test/index.mjs';
import {spawn, spawnSync} from 'node:child_process';
import {randomBytes, createHash} from 'node:crypto';
import {mkdir, mkdtemp, writeFile, readFile, rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join, resolve} from 'node:path';
import net from 'node:net';

const root = resolve('.');
const evidence = resolve(process.env.KINOSAIL_SIGNIN_EVIDENCE);
const state = await mkdtemp(join(tmpdir(), 'kinosail-signin-synthetic-'));
const git = (...args) => spawnSync('git', args, {encoding:'utf8'}).stdout.trim();
const digest = value => createHash('sha256').update(value).digest('hex');
const receipt = {
  revision:git('rev-parse', 'HEAD'), workingDiffSHA256:digest(git('diff', 'HEAD')),
  command:'python3 scripts/testing/test-session-persistence-local.py',
  environment:`${process.platform}/${process.arch}; actual native Go apps; loopback HTTP; persistent Chromium`,
  data:'Disposable synthetic Owners and virtual WebAuthn; no production accounts',
  boundaries:'Real browser/server persistence and HTTP authorization. Virtual WebAuthn verifies the protocol, not a physical passkey or Safari. No TLS bypass or production writes.',
  results:[], result:'failed',
};
const apps = [];
let context;
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const freePort = () => new Promise(resolve => {
  const server = net.createServer();
  server.listen(0, '127.0.0.1', () => {
    const port = server.address().port;
    server.close(() => resolve(port));
  });
});
async function ready(app) {
  for (let attempt = 0; attempt < 100; attempt++) {
    if (app.process.exitCode !== null) throw new Error('Disposable Server exited before readiness');
    try { if ((await fetch(app.url+'/healthz')).ok) return; } catch {}
    await sleep(100);
  }
  throw new Error('Disposable Server readiness failed');
}
function start(app) {
  app.process = spawn(join(evidence, app.name), [], {env:{...process.env,
    KINOSAIL_LISTEN:`127.0.0.1:${app.port}`, KINOSAIL_AUTH_URL:app.url, KINOSAIL_TLS_ENABLED:'false',
    KINOSAIL_DATA_DIR:app.config, KINOSAIL_MEDIA_DIR:join(state, 'media'),
    KINOSAIL_CACHE_DIR:join(state, app.name, 'cache'), KINOSAIL_BACKUP_DIR:join(state, app.name, 'backups'),
    KINOSAIL_BACKUP_KEY:randomBytes(32).toString('hex')}, stdio:'ignore'});
}
async function stop(app) {
  if (!app.process) return;
  if (app.process.exitCode === null && app.process.signalCode === null) {
    const exited = new Promise(resolve => app.process.once('exit', resolve));
    app.process.kill('SIGTERM');
    await exited;
  }
  app.process = undefined;
}
const safeCookie = cookie => ({name:cookie.name, domain:cookie.domain, path:cookie.path,
  secure:cookie.secure, httpOnly:cookie.httpOnly, sameSite:cookie.sameSite,
  persistent:cookie.expires>0, secondsLeft:Math.round(cookie.expires-Date.now()/1000)});
async function openBrowser() {
  context = await chromium.launchPersistentContext(join(state, 'browser'), {
    ...(process.platform==='darwin' ? {channel:'chrome'} : {}), headless:true, ignoreHTTPSErrors:false,
    serviceWorkers:'block',
  });
  receipt.browserVersion = context.browser()?.version();
  await context.addInitScript(() => Object.defineProperty(PublicKeyCredential,
    'isConditionalMediationAvailable', {value:async()=>false}));
}
async function tab(app) {
  const page = await context.newPage();
  app.begins = 0;
  page.on('request', request => {
    if (new URL(request.url()).pathname==='/api/v1/passkeys/login/begin') app.begins++;
  });
  return page;
}
async function virtual(page) {
  const client = await context.newCDPSession(page);
  await client.send('WebAuthn.enable');
  await client.send('WebAuthn.addVirtualAuthenticator', {options:{protocol:'ctap2', transport:'internal',
    hasResidentKey:true, hasUserVerification:true, isUserVerified:true, automaticPresenceSimulation:true}});
}
async function setup(app) {
  const page = await tab(app);
  await virtual(page);
  await page.goto(app.url+'/setup');
  await page.getByLabel('Name', {exact:true}).fill('Synthetic Owner');
  await page.locator('#new-password').fill(randomBytes(24).toString('hex'));
  await page.getByLabel(/Add extra sign-in protection now/).uncheck();
  await page.getByRole('button', {name:'Create Owner & continue'}).click();
  await page.getByRole('button', {name:'Create passkey', exact:true}).click();
  await expect(page).toHaveURL(app.url+'/onboarding/connection');
  await page.goto(app.url+'/onboarding/finish');
  await page.goto(app.url+'/');
  await page.evaluate(async () => {
    const csrf = document.querySelector('meta[name="kinosail-csrf"]').content;
    const response = await fetch('/api/v1/session', {method:'DELETE', headers:{'X-Kinosail-CSRF':csrf}});
    if (response.status!==204) throw new Error('Synthetic sign-out failed');
  });
  await page.goto(app.url+'/login');
  await expect(page).toHaveURL(app.url+'/');
  expect(app.begins).toBeGreaterThan(0);
  const cookies = (await context.cookies(app.url)).filter(cookie => cookie.name===app.cookie);
  expect(cookies.length).toBe(1);
  expect(cookies[0].secure && cookies[0].httpOnly && cookies[0].sameSite==='Strict' && cookies[0].expires>0).toBe(true);
  receipt.results.push({app:app.name, journey:'actual WebAuthn sign-in', result:'passed', cookie:safeCookie(cookies[0])});
  await page.close();
}
async function observe(app, path, journey) {
  const page = await tab(app);
  await page.goto(app.url+path);
  await expect(page).not.toHaveURL(/\/login(?:\?|$)/);
  expect(await page.locator('[data-passkey-login]').count()).toBe(0);
  const status = await page.evaluate(async () => (await fetch('/api/v1/me')).status);
  expect(status).toBe(200);
  expect(app.begins).toBe(0);
  const finalPath = new URL(page.url()).pathname;
  if (path.includes('next=')) expect(finalPath).toBe('/account');
  receipt.results.push({app:app.name, journey, requestedPath:path, result:'passed', finalPath, meStatus:status, passkeyBegins:app.begins});
  await page.screenshot({path:join(evidence, `${app.name}-${journey.replaceAll(' ', '-')}.png`)});
  await page.close();
}
async function external(app) {
  const page = await tab(app);
  const navigations = [];
  page.on('request', async request => {
    const url = new URL(request.url());
    if (request.isNavigationRequest() && url.origin===app.url) {
      const headers = await request.allHeaders();
      navigations.push({path:url.pathname, cookieSent:(headers.cookie||'').includes(app.cookie+'=')});
    }
  });
  await page.route('http://127.0.0.1:39999/**', route => route.fulfill({contentType:'text/html',
    body:`<a href="${app.url}/">Open app</a>`}));
  await page.goto('http://127.0.0.1:39999/');
  await page.getByRole('link', {name:'Open app'}).click();
  await expect(page).toHaveURL(app.url+'/');
  expect(await page.evaluate(async () => (await fetch('/api/v1/me')).status)).toBe(200);
  expect(app.begins).toBe(0);
  receipt.results.push({app:app.name, journey:'cross-site reopening with Strict cookie', result:'passed', navigations, passkeyBegins:app.begins});
  await page.screenshot({path:join(evidence, app.name+'-external-navigation.png')});
  await page.close();
}
async function rejection(app, journey) {
  const page = await tab(app);
  await page.goto(app.url+'/login');
  await expect(page.locator('[data-passkey-login]')).toBeVisible();
  expect(new URL(page.url()).pathname).toBe('/login');
  const status = await page.evaluate(async () => (await fetch('/api/v1/me')).status);
  expect(status).toBe(401);
  receipt.results.push({app:app.name, journey, result:'passed', meStatus:status, finalPath:'/login'});
  await page.close();
}
const boundaryScript = `import json, sqlite3, sys, time
path, mode, edge = sys.argv[1:]
assert path.startswith('/tmp/kinosail-signin-synthetic-') or '/kinosail-signin-synthetic-' in path
conn = sqlite3.connect(path+'/kinosail.db')
row = conn.execute("SELECT value FROM state WHERE name='settings.json'").fetchone()
policy = json.loads(row[0]); idle = int((policy.get('sessionInactiveHours') or .25)*3600); absolute = int((policy.get('sessionAbsoluteHours') or 8)*3600)
row = conn.execute("SELECT value FROM state WHERE name='sessions.json'").fetchone(); values = json.loads(row[0]); now = int(time.time())
count = 0
for session in values.values():
    if not session.get('browser'): continue
    count += 1; session['createdAt'] = now; session['lastSeen'] = now; session['expiresAt'] = now+absolute
    margin = 30 if edge=='before' else 0
    if mode=='idle': session['lastSeen'] = now-idle+margin
    elif mode=='absolute': session['createdAt'] = now-absolute+margin
    elif mode=='expiry': session['expiresAt'] = now+margin
assert count==1
conn.execute("UPDATE state SET value=? WHERE name='sessions.json'", (json.dumps(values).encode(),)); conn.commit(); conn.close()
print(json.dumps({'mode':mode, 'edge':edge, 'fixtureUnix':now, 'idleSeconds':idle, 'absoluteSeconds':absolute, 'marginSeconds':margin}))`;
async function expiry(app, mode) {
  await stop(app);
  app.config = join(state, app.name, mode, 'config');
  start(app); await ready(app); await setup(app);
  for (const edge of ['before', 'at']) {
    await stop(app);
    const projection = spawnSync('python3', ['-c', boundaryScript, app.config, mode, edge], {encoding:'utf8'});
    if (projection.status!==0) throw new Error('Disposable expiry fixture preparation failed');
    receipt.results.push({app:app.name, journey:'synthetic boundary fixture', ...JSON.parse(projection.stdout)});
    start(app); await ready(app);
    if (edge==='before') await observe(app, '/', mode+' before boundary');
    else await rejection(app, mode+' at boundary');
  }
}
try {
  await mkdir(join(state, 'media'));
  for (const [name, cookie] of [['player', '__Host-kinosail_player_session'], ['subtitles', '__Host-kinosail_subtitles_session']]) {
    const app = {name, cookie, port:await freePort(), config:join(state, name, 'config')};
    app.url = `http://localhost:${app.port}`;
    apps.push(app); start(app); await ready(app);
  }
  await openBrowser();
  for (const app of apps) await setup(app);
  for (const app of apps) {
    await observe(app, '/', 'tab reopen library');
    await observe(app, '/login', 'tab reopen saved login');
    await observe(app, '/login?next=%2Faccount', 'tab reopen login return');
    await external(app);
  }
  await context.close(); await openBrowser();
  for (const app of apps) {
    await observe(app, '/login', 'browser restart saved login');
    await stop(app); start(app); await ready(app);
    await observe(app, '/login', 'server restart saved login');
  }
  for (const app of apps) for (const mode of ['idle', 'absolute', 'expiry']) await expiry(app, mode);
  receipt.result = 'passed';
} catch (error) {
  receipt.error = String(error.message).slice(0, 700);
} finally {
  if (context) await context.close();
  for (const app of apps) await stop(app);
  receipt.binarySHA256 = {};
  for (const app of apps) receipt.binarySHA256[app.name] = digest(await readFile(join(evidence, app.name)));
  await writeFile(join(evidence, 'receipt.json'), JSON.stringify(receipt, null, 2)+'\n');
  await rm(state, {recursive:true, force:true});
  console.log(JSON.stringify({artifact:evidence, result:receipt.result, observations:receipt.results.length, error:receipt.error}, null, 2));
}
process.exitCode = receipt.result==='passed' ? 0 : 1;
