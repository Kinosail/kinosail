const {chromium, expect} = await import(process.env.KINOSAIL_PLAYWRIGHT_MODULE ?? '../../apps/player/e2e/node_modules/@playwright/test/index.mjs');
import {spawn, spawnSync} from 'node:child_process';
import {randomBytes, createHash} from 'node:crypto';
import {mkdir, mkdtemp, writeFile, readFile, rm} from 'node:fs/promises';
import {tmpdir} from 'node:os';
import {join, resolve} from 'node:path';
import net from 'node:net';

const root = resolve('.');
const evidence = resolve(process.env.KINOSAIL_TIMEOUT_EVIDENCE);
const state = await mkdtemp(join(tmpdir(), 'kinosail-timeout-synthetic-'));
const git = (...args) => spawnSync('git', args, {encoding:'utf8'}).stdout.trim();
const digest = value => createHash('sha256').update(value).digest('hex');
const receipt = {
  revision:git('rev-parse', 'HEAD'), workingDiffSHA256:digest(git('diff', 'HEAD')),
  command:'python3 scripts/testing/test-public-timeouts-local.py',
  environment:`${process.platform}/${process.arch}; actual native Go apps; loopback HTTP; persistent Chromium`,
  data:'Disposable synthetic Owners and virtual WebAuthn; no production accounts',
  boundaries:'Real browser/server persistence and HTTP authorization. Virtual WebAuthn verifies the protocol, not a physical passkey or Safari. No TLS bypass or production writes.',
  results:[], result:'failed',
};
const apps = [];
let context;
const viewerContexts=[];
const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
const freePort = () => new Promise(resolve => {
  const server = net.createServer();
  server.listen(0, '127.0.0.1', () => {
    const port = server.address().port;
    server.close(() => resolve(port));
  });
});
async function ready(app) {
  for (let attempt = 0; attempt < 300; attempt++) {
    if (app.process.exitCode !== null) throw new Error('Disposable Server exited before readiness');
    try { if ((await fetch(app.url+'/healthz')).ok) return; } catch {}
    await sleep(100);
  }
  throw new Error('Disposable Server readiness failed');
}
function start(app) {
  app.process = spawn(join(evidence, app.name+(app.legacy?'-legacy':'')), [], {env:{...process.env,
    KINOSAIL_LISTEN:`127.0.0.1:${app.port}`, KINOSAIL_AUTH_URL:app.url, KINOSAIL_TLS_ENABLED:'false',
    KINOSAIL_DATA_DIR:app.config, KINOSAIL_MEDIA_DIR:join(state, 'media'),
    KINOSAIL_CACHE_DIR:join(state, app.name, 'cache'), KINOSAIL_BACKUP_DIR:join(state, app.name, 'backups'),
    KINOSAIL_BACKUP_KEY:randomBytes(32).toString('hex'), KINOSAIL_PROXY_TOKEN:app.proxy}, stdio:['ignore','pipe','pipe']});
  const capture=chunk=>{ for(const line of chunk.toString().split('\n')) if(line.includes('session timeout settings persistence failed')||line.includes('route=')&&line.includes('/settings/public-session-timeouts')) app.safeLogs.push(line.slice(0,700)); };
  app.process.stdout.on('data',capture); app.process.stderr.on('data',capture);
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
  app.owner = page; app.lastpage=page;
}
async function api(page, path, method='GET', body, csrf=true) {
  return page.evaluate(async ({path,method,body,csrf}) => {
    const headers={'Content-Type':'application/json'};
    if(csrf) headers['X-Kinosail-CSRF']=document.querySelector('meta[name="kinosail-csrf"]')?.content || '';
    const response=await fetch(path,{method,headers,body:body===undefined?undefined:JSON.stringify(body)});
    return {status:response.status,body:await response.json().catch(()=>({}))};
  },{path,method,body,csrf});
}
const record=(app,journey,extra={})=>{ receipt.results.push({app:app.name,journey,result:'passed',...extra}); console.log(app.name+': '+journey); };
const settingsPath=publicAccess=>`/api/v1/settings/${publicAccess?'public-session-timeouts':'session-timeouts'}`;
async function save(app,publicAccess,inactive,absolute) {
  const response=await api(app.owner,settingsPath(publicAccess),'PUT',{inactiveHours:inactive,absoluteHours:absolute});
  expect(response.status).toBe(200); record(app,'Owner API saves '+(publicAccess?'public':'private'),{inactive,absolute});
}
async function fixture(app,operation,payload) {
  await stop(app);
  const result=spawnSync('python3',['scripts/testing/public-timeouts-fixture.py',app.config,operation,JSON.stringify(payload)],{encoding:'utf8'});
  if(result.status!==0) throw new Error('Disposable time fixture failed: '+result.stderr.slice(-400));
  record(app,'stopped synthetic fixture',JSON.parse(result.stdout));
  start(app); await ready(app);
}
async function me(app,client,status,journey) {
  expect((await api(client.page,'/api/v1/me')).status).toBe(status); record(app,journey,{status});
}
async function viewer(app) {
  app.viewerPassword=randomBytes(24).toString('hex');
  const created=await api(app.owner,'/api/v1/profiles','POST',{name:'Synthetic Viewer',password:app.viewerPassword,rating:'all',libraries:['all'],remote:true});
  expect(created.status).toBe(201); app.viewerID=created.body.id;
  const browser=await chromium.launch({...(process.platform==='darwin'?{channel:'chrome'}:{}),headless:true});
  app.viewerBrowser=browser;
  const ctx=await browser.newContext({ignoreHTTPSErrors:false}); viewerContexts.push(ctx);
  const page=await ctx.newPage();
  await page.goto(app.url+'/login'); await page.getByLabel('Name',{exact:true}).fill('Synthetic Viewer');
  await page.getByLabel('Password',{exact:true}).fill(app.viewerPassword);
  await page.getByRole('button',{name:'Sign in',exact:true}).click();
  await page.goto(app.url+'/account'); app.lastpage=page;
  const cdp=await ctx.newCDPSession(page); await cdp.send('WebAuthn.enable');
  const {authenticatorId}=await cdp.send('WebAuthn.addVirtualAuthenticator',{options:{protocol:'ctap2',transport:'internal',hasResidentKey:true,hasUserVerification:true,isUserVerified:true,automaticPresenceSimulation:true}});
  await page.locator('[data-passkey-add]').click();
  await expect(page.locator('[data-passkey-status]')).toContainText('Passkey');
  const {credentials}=await cdp.send('WebAuthn.getCredentials',{authenticatorId});
  app.credentials=credentials; await ctx.close();
}
async function signIn(app,publicAccess) {
  const ctx=await app.viewerBrowser.newContext({ignoreHTTPSErrors:false,
    ...(publicAccess?{extraHTTPHeaders:{'X-Kinosail-Proxy-Token':app.proxy}}:{})});
  viewerContexts.push(ctx);
  await ctx.addInitScript(()=>Object.defineProperty(PublicKeyCredential,'isConditionalMediationAvailable',{value:async()=>false}));
  const page=await ctx.newPage(); app.lastpage=page; const cdp=await ctx.newCDPSession(page);
  await cdp.send('WebAuthn.enable');
  const {authenticatorId}=await cdp.send('WebAuthn.addVirtualAuthenticator',{options:{protocol:'ctap2',transport:'internal',hasResidentKey:true,hasUserVerification:true,isUserVerified:true,automaticPresenceSimulation:true}});
  for(const credential of app.credentials) await cdp.send('WebAuthn.addCredential',{authenticatorId,credential});
  await page.goto(app.url+'/login'); if(publicAccess) await page.getByText('Already have another sign-in method?',{exact:true}).click(); await page.locator('[data-passkey-login]').click(); await expect(page).toHaveURL(app.url+'/');
  const cookies=(await ctx.cookies(app.url)).filter(value=>value.name===app.cookie);
  expect(cookies).toHaveLength(1);
  expect(cookies[0].secure&&cookies[0].httpOnly&&cookies[0].sameSite==='Strict').toBe(true);
  record(app,'actual '+(publicAccess?'public':'private')+' WebAuthn issuance',{cookie:safeCookie(cookies[0])});
  app.credentials=(await cdp.send('WebAuthn.getCredentials',{authenticatorId})).credentials;
  return {ctx,page,cookie:safeCookie(cookies[0])};
}
async function permissionChecks(app,local,pub) {
  const before=(await api(app.owner,'/api/v1/settings')).body;
  for(const client of [local,pub]) for(const publicAccess of [false,true]) {
    const response=await api(client.page,settingsPath(publicAccess),'PUT',{inactiveHours:1,absoluteHours:4});
    expect(response.status).toBe(client===pub&&app.name==='player'?404:403); record(app,'Viewer settings denied',{publicSession:client===pub,publicPolicy:publicAccess,status:response.status});
  }
  expect((await api(app.owner,settingsPath(true),'PUT',{inactiveHours:1,absoluteHours:4},false)).status).toBe(403);
  expect((await api(app.owner,settingsPath(true),'DELETE',undefined,false)).status).toBe(403);
  expect((await api(app.owner,settingsPath(true),'DELETE',{invalid:true})).status).toBe(400);
  expect((await api(local.page,settingsPath(true),'DELETE')).status).toBe(403);
  expect((await api(pub.page,settingsPath(true),'DELETE')).status).toBe(app.name==='player'?404:403);
  record(app,'missing CSRF and unauthorized/reset bodies denied');
  for(const input of [{inactiveHours:0,absoluteHours:8},{inactiveHours:-1,absoluteHours:8},{inactiveHours:.24,absoluteHours:8},{inactiveHours:1,absoluteHours:3.99},{inactiveHours:8761,absoluteHours:8761},{inactiveHours:24,absoluteHours:8},{inactiveHours:1,absoluteHours:8761},{inactiveHours:1,absoluteHours:8,unknown:true}]) {
    expect((await api(app.owner,settingsPath(true),'PUT',input)).status).toBe(400);
  }
  await app.owner.goto(app.url+'/settings#security');
  const webBad=await app.owner.evaluate(async()=>{
    const csrf=document.querySelector('meta[name="kinosail-csrf"]').content;
    const responses=[];
    for(const body of ['inactiveHours=NaN&absoluteHours=8','inactiveHours=Infinity&absoluteHours=8760','inactiveHours=1&inactiveHours=8&absoluteHours=8','inactiveHours=1&absoluteHours=8&unknown=1']) {
      responses.push((await fetch('/settings/public-session-timeouts',{method:'POST',headers:{'Content-Type':'application/x-www-form-urlencoded','X-Kinosail-CSRF':csrf},body})).status);
    }
    return responses;
  }); expect(webBad).toEqual([400,400,400,400]);
  const after=(await api(app.owner,'/api/v1/settings')).body;
  for(const key of ['sessionInactiveHours','sessionAbsoluteHours','publicSessionInactiveHours','publicSessionAbsoluteHours']) expect(after[key]).toBe(before[key]);
  record(app,'invalid boundaries reject without changing policies');
  const ownerID=(await api(app.owner,'/api/v1/me')).body.viewer.id;
  await fixture(app,'stale-owner',{ownerID});
  expect((await api(app.owner,settingsPath(true),'PUT',{inactiveHours:1,absoluteHours:4})).status).toBe(403);
  record(app,'stale Owner step-up required'); await fixture(app,'restore-owner',{ownerID});
}
async function boundaryChecks(app) {
  await save(app,false,1,4); await save(app,true,8,24);
  for(const publicAccess of [false,true]) for(const mode of ['idle','absolute','expiry']) {
    const client=await signIn(app,publicAccess); const inactive=publicAccess?8:1, absolute=publicAccess?24:4;
    for(const [margin,status] of [[30,200],[0,401]]) {
      await fixture(app,'boundary',{viewerID:app.viewerID,channel:publicAccess?'public':'',mode,inactive,absolute,margin});
      await me(app,client,status,`${publicAccess?'public':'private'} ${mode} ${margin?'before':'at'} boundary`);
    }
    await client.ctx.close();
  }
}
async function transitionChecks(app) {
  await save(app,true,8760,8760); const pub=await signIn(app,true);
  expect(pub.cookie.secondsLeft).toBeGreaterThan(8760*3600-60);
  await fixture(app,'boundary',{viewerID:app.viewerID,channel:'public',mode:'age',inactive:8760,absolute:8760});
  await me(app,pub,200,'public session accepted after 33 days within one year');
  await save(app,true,1,4); await me(app,pub,401,'tightening past absolute removes old session');
  await save(app,true,8760,8760); await me(app,pub,401,'raising does not resurrect removed session'); await pub.ctx.close();
  await save(app,true,1,4); const unobserved=await signIn(app,true);
  await fixture(app,'boundary',{viewerID:app.viewerID,channel:'public',mode:'absolute',inactive:1,absolute:4,margin:0,preserveCeilings:true});
  await save(app,true,8760,8760); await me(app,unobserved,401,'raising removes unobserved already-expired session'); await unobserved.ctx.close();
  await save(app,true,1,4); const pinned=await signIn(app,true);
  await save(app,true,8760,8760); await me(app,pinned,200,'raising preserves still-valid session');
  await fixture(app,'boundary',{viewerID:app.viewerID,channel:'public',mode:'idle',inactive:1,absolute:4,margin:0,preserveCeilings:true});
  await me(app,pinned,401,'old issued idle ceiling remains after raising'); await pinned.ctx.close();
  await save(app,true,8,24); const valid=await signIn(app,true);
  await save(app,true,1,4); await me(app,valid,200,'tightening preserves still-valid session'); await valid.ctx.close();
}
const {extraJourneys}=await import('./public-timeouts-journeys.mjs');
const {configUI,legacyChecks,admissionChecks,atomicChecks,uiStates,enableMFAConcurrency,rollbackChecks}=extraJourneys({api,expect,record,fixture,signIn,me,save,stop,start,ready,spawnSync,settingsPath,join,evidence});
try {
  await mkdir(join(state,'media'));
  for(const [name,cookie] of [['player','__Host-kinosail_player_session'],['subtitles','__Host-kinosail_subtitles_session']]) {
    if(process.env.KINOSAIL_TIMEOUT_APP && name!==process.env.KINOSAIL_TIMEOUT_APP) continue;
    const app={name,cookie,safeLogs:[],proxy:randomBytes(24).toString('hex'),port:await freePort(),config:join(state,name,'config')};
    app.url=`http://localhost:${app.port}`; apps.push(app); start(app); await ready(app);
  }
  await openBrowser();
  for(const app of apps) {
    await setup(app); const settings=await api(app.owner,'/api/v1/settings'); expect(settings.status).toBe(200);
    expect(settings.body.publicSessionInactiveHours).toBe(.25); expect(settings.body.publicSessionAbsoluteHours).toBe(8);
    expect(settings.body.publicSessionTimeoutsConfigured).toBe(false); record(app,'legacy effective public defaults');
    await configUI(app); await uiStates(app); if(process.env.KINOSAIL_TIMEOUT_PHASE==='ui') continue; await viewer(app);
    const local=await signIn(app,false), pub=await signIn(app,true);
    await permissionChecks(app,local,pub);
    await stop(app); start(app); await ready(app);
    const persisted=(await api(app.owner,'/api/v1/settings')).body;
    expect(persisted.publicSessionInactiveHours).toBe(8760); expect(persisted.sessionInactiveHours).toBe(.25);
    await me(app,local,200,'private persists across restart'); await me(app,pub,200,'public persists across restart');
    record(app,'independent settings persist across server restart'); await local.ctx.close(); await pub.ctx.close();
    await boundaryChecks(app); await transitionChecks(app); await atomicChecks(app); await legacyChecks(app); await admissionChecks(app); await rollbackChecks(app); await enableMFAConcurrency(app);
  }
  receipt.result='passed';
} catch(error) { receipt.error=String(error.message).slice(0,700); for(const app of apps) if(app.lastpage&&!app.lastpage.isClosed()) { await app.lastpage.screenshot({path:join(evidence,app.name+'-failure.png')}); receipt.failureUI=(await app.lastpage.locator('main').innerText().catch(()=> '')).slice(0,1500); } }
finally {
  for(const ctx of viewerContexts) await ctx.close().catch(()=>{});
  if(context) await context.close();
  for(const app of apps) { if(app.viewerBrowser) await app.viewerBrowser.close(); await stop(app); }
  receipt.safeServerLogs=Object.fromEntries(apps.map(app=>[app.name,app.safeLogs]));
  receipt.binarySHA256={};
  for(const app of apps) { receipt.binarySHA256[app.name]=digest(await readFile(join(evidence,app.name))); receipt.binarySHA256[app.name+'-legacy']=digest(await readFile(join(evidence,app.name+'-legacy'))); }
  await writeFile(join(evidence,'receipt.json'),JSON.stringify(receipt,null,2)+'\n');
  await rm(state,{recursive:true,force:true});
  console.log(JSON.stringify({artifact:evidence,result:receipt.result,observations:receipt.results.length,error:receipt.error},null,2));
}
process.exitCode=receipt.result==='passed'?0:1;
