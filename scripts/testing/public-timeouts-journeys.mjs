export function extraJourneys(deps) {
const {api,expect,record,fixture,signIn,me,save,stop,start,ready,spawnSync,settingsPath,join,evidence}=deps;
async function configUI(app) {
  const page=app.owner; app.lastpage=page;
  for(const width of [1920,1440,1024,390,320]) {
    await page.setViewportSize({width,height:900}); const response=await page.goto(app.url+'/settings#security');
    if(response) expect(response.status()).toBe(200);
    const section=page.locator('#session-timeouts'); await expect(section).toBeVisible();
    for(const access of ['private','public']) {
      const form=section.locator(`[data-timeout-access="${access}"]`);
      await expect(form.getByLabel('After inactivity').locator('option')).toHaveText(['15 minutes (recommended)','1 hour','8 hours','1 day','3 days','7 days','30 days','90 days','1 year']);
      await expect(form.getByLabel('Always after').locator('option')).toHaveText(['4 hours','8 hours (recommended)','1 day','7 days','30 days','90 days','1 year']);
    }
    expect(await section.evaluate(el=>el.scrollWidth<=el.clientWidth+1&&el.getBoundingClientRect().right<=document.documentElement.clientWidth+1)).toBe(true);
    const {default:AxeBuilder}=await import('../../apps/player/e2e/node_modules/@axe-core/playwright/dist/index.mjs');
    expect((await new AxeBuilder({page}).include('#session-timeouts').analyze()).violations).toEqual([]);
    await page.screenshot({path:join(evidence,`${app.name}-settings-${width}.png`),fullPage:true});
    record(app,'responsive settings',{width});
  }
  const form=page.locator('[data-timeout-access="public"]');
  await form.getByLabel('After inactivity').selectOption('8760'); await form.getByLabel('Always after').selectOption('8760');
  await form.getByRole('button',{name:'Save public timeouts'}).click();
  await expect(form.getByLabel('After inactivity')).toHaveValue('8760');
  const fields=(await api(page,'/api/v1/settings')).body;
  expect(fields.publicSessionInactiveHours).toBe(8760); expect(fields.publicSessionAbsoluteHours).toBe(8760);
  expect(fields.sessionInactiveHours).toBe(.25); expect(fields.sessionAbsoluteHours).toBe(8);
  record(app,'Owner selects one year public through real form');
  const privateForm=page.locator('[data-timeout-access="private"]');
  await privateForm.getByLabel('After inactivity').selectOption('1'); await privateForm.getByLabel('Always after').selectOption('4');
  await privateForm.getByRole('button',{name:'Save private timeouts'}).click();
  const independent=(await api(page,'/api/v1/settings')).body;
  expect(independent.sessionInactiveHours).toBe(1); expect(independent.sessionAbsoluteHours).toBe(4);
  expect(independent.publicSessionInactiveHours).toBe(8760); expect(independent.publicSessionAbsoluteHours).toBe(8760);
  record(app,'Owner selects private form without changing public'); await save(app,false,.25,8);
}
async function snapshot(app) {
  await stop(app);
  const result=spawnSync('python3',['scripts/testing/public-timeouts-fixture.py',app.config,'snapshot',JSON.stringify({viewerID:app.viewerID})],{encoding:'utf8'});
  if(result.status!==0) throw new Error('Synthetic safe snapshot failed');
  const projection=JSON.parse(result.stdout); start(app); await ready(app); return projection;
}
async function legacyChecks(app) {
  for(const [inactive,absolute] of [[.25,8],[24,168],[1,4]]) {
    await fixture(app,'legacy',{inactive,absolute});
    const policy=(await api(app.owner,'/api/v1/settings')).body;
    expect(policy.publicSessionInactiveHours).toBe(inactive); expect(policy.publicSessionAbsoluteHours).toBe(Math.min(absolute,8));
    expect(policy.publicSessionTimeoutsConfigured).toBe(false);
    const client=await signIn(app,true); expect(client.cookie.secondsLeft).toBeGreaterThan(8*3600-60); expect(client.cookie.secondsLeft).toBeLessThanOrEqual(8*3600);
    for(const [margin,status] of [[30,200],[0,401]]) {
      await fixture(app,'boundary',{viewerID:app.viewerID,channel:'public',mode:'absolute',inactive,absolute:Math.min(absolute,8),margin,legacy:true});
      await me(app,client,status,'legacy private '+inactive+'/'+absolute+' public absolute '+(margin?'before':'at'));
    }
    await client.ctx.close(); record(app,'legacy validation and cookie issuance preserved',{inactive,absolute});
  }
}
async function admissionChecks(app) {
  await save(app,true,.25,8760);
  const clients=[];
  for(let count=0;count<10;count++) clients.push(await signIn(app,true));
  await fixture(app,'boundary',{viewerID:app.viewerID,channel:'public',mode:'idle',inactive:.25,absolute:8760,margin:0});
  const next=await signIn(app,true); await me(app,next,200,'ten unobserved idle-expired public sessions release sign-in capacity');
  for(const client of [...clients,next]) await client.ctx.close();
}
async function atomicChecks(app) {
  await save(app,true,8,24); const client=await signIn(app,true);
  const before=await snapshot(app); await fixture(app,'deny-write',{});
  expect((await api(app.owner,settingsPath(true),'PUT',{inactiveHours:1,absoluteHours:4})).status).toBe(500);
  const after=await snapshot(app); expect(after).toEqual(before);
  record(app,'SQLite failure rolls back settings and session ceilings');
  await fixture(app,'allow-write',{}); await me(app,client,200,'failed Save retains existing session'); await client.ctx.close();
  const responses=await app.owner.evaluate(async()=>{
    const csrf=document.querySelector('meta[name="kinosail-csrf"]').content;
    return Promise.all(Array.from({length:8},(_,i)=>fetch(i%2?'/api/v1/settings/mfa':'/api/v1/settings/public-session-timeouts',{
      method:'PUT',headers:{'Content-Type':'application/json','X-Kinosail-CSRF':csrf},body:JSON.stringify(i%2?{required:true}:{inactiveHours:8,absoluteHours:24})
    }).then(response=>response.status)));
  }); expect(responses).toEqual(Array(8).fill(200)); record(app,'concurrent MFA and timeout Save complete');
}
async function uiStates(app) {
  const page=app.owner; app.lastpage=page;
  await page.goto(app.url+'/settings#security');
  const section=page.locator('#session-timeouts'), form=section.locator('[data-timeout-access="public"]');
  await page.setViewportSize({width:1440,height:900}); await form.getByRole('button',{name:'Save public timeouts'}).scrollIntoViewIfNeeded();
  const loaded=await section.boundingBox();
  let pending, resume;
  const seen=new Promise(resolve=>pending=resolve), release=new Promise(resolve=>resume=resolve);
  await page.route(app.url+'/settings/public-session-timeouts',async route=>{pending(); await release; await route.continue();});
  let responseError; const saved=page.waitForResponse(response=>response.url()===app.url+'/settings/public-session-timeouts'&&response.request().method()==='POST').catch(error=>{responseError=error;resume();});
  let clickError; const clicked=form.getByRole('button',{name:'Save public timeouts'}).click({noWaitAfter:true}).catch(error=>{clickError=error;pending();resume();});
  await seen; if(clickError) throw clickError;
  record(app,'actual form request pending; browser owns full-page navigation');
  resume(); await clicked; await saved; if(responseError) throw responseError; await page.unroute(app.url+'/settings/public-session-timeouts');
  await expect(section).toBeVisible(); const after=await section.boundingBox();
  expect({width:after.width,height:after.height}).toEqual({width:loaded.width,height:loaded.height});
  record(app,'loaded controls keep geometry after pending Save; no custom skeleton');
  await fixture(app,'deny-write',{});
  const failed=page.waitForResponse(response=>response.url()===app.url+'/settings/public-session-timeouts'&&response.request().method()==='POST');
  await form.getByRole('button',{name:'Save public timeouts'}).click();
  expect((await failed).status()).toBe(500);
  await expect(page.locator('main')).toContainText('could not save session timeouts');
  await page.screenshot({path:join(evidence,app.name+'-settings-failed.png'),fullPage:true});
  await fixture(app,'allow-write',{}); await page.goto(app.url+'/settings#security');
  await expect(form.getByLabel('Always after')).toHaveValue('8760');
  record(app,'failed form Save retains policy and recovers loaded controls');
}
async function enableMFAConcurrency(app) {
  expect((await api(app.owner,'/api/v1/settings/mfa','PUT',{required:false})).status).toBe(200);
  const responses=await app.owner.evaluate(async()=>{
    const csrf=document.querySelector('meta[name="kinosail-csrf"]').content;
    return Promise.all(Array.from({length:8},(_,i)=>fetch(i%2?'/api/v1/settings/mfa':'/api/v1/settings/public-session-timeouts',{
      signal:AbortSignal.timeout(5000),method:'PUT',headers:{'Content-Type':'application/json','X-Kinosail-CSRF':csrf},
      body:JSON.stringify(i%2?{required:true}:{inactiveHours:8,absoluteHours:24})
    }).then(response=>({mfa:!!(i%2),status:response.status}))));
  });
  expect(responses.some(value=>value.mfa&&value.status===200)).toBe(true);
  for(const response of responses) expect([200,401,403]).toContain(response.status);
  const after=await snapshot(app); expect(after.requiredMFA).toBe(true); expect(after.sessions).toEqual([]);
  record(app,'disabled to enabled MFA concurrent timeout Save completes and revokes sessions',{statuses:responses});
}
async function rollbackChecks(app) {
  await save(app,true,8760,8760); const removed=await signIn(app,true);
  await fixture(app,'boundary',{viewerID:app.viewerID,channel:'public',mode:'expiry',inactive:8760,absolute:8760,margin:0});
  await save(app,true,8760,8760); await me(app,removed,401,'unobserved expired sessions removed before rollback');
  const valid=await signIn(app,true), before=await snapshot(app);
  expect((await api(app.owner,settingsPath(true),'DELETE')).status).toBe(200);
  const fields=(await api(app.owner,'/api/v1/settings')).body;
  expect(fields.publicSessionTimeoutsConfigured).toBe(false);
  const reset=await snapshot(app); expect(reset.compatibleSettingsSHA256).toBe(before.compatibleSettingsSHA256);
  expect(reset.unrelatedDataSHA256).toEqual(before.unrelatedDataSHA256);
  for(const session of reset.sessions.filter(value=>value.channel==='public')) expect(session.expiresAt-session.createdAt).toBeLessThanOrEqual(8*3600);
  expect((await api(app.owner,'/api/v1/sessions','DELETE')).status).toBe(204);
  expect((await api(app.owner,'/api/v1/session','DELETE')).status).toBe(204);
  expect((await api(app.owner,'/api/v1/me')).status).toBe(401);
  await stop(app); app.legacy=true; start(app); await ready(app);
  await me(app,valid,401,'pre-feature binary cannot reuse rollback-revoked session');
  await app.owner.evaluate(()=>localStorage.removeItem('kinosail-passkey'));
  const begins=app.begins;
  await app.owner.goto(app.url+'/login'); await app.owner.locator('[data-passkey-login]').click(); await expect(app.owner).toHaveURL(app.url+'/');
  expect(app.begins).toBeGreaterThan(begins);
  expect((await api(app.owner,'/api/v1/settings')).status).toBe(200);
  record(app,'pre-feature binary reads current DB and accepts fresh Owner sign-in');
  await me(app,removed,401,'pre-feature binary cannot resurrect deleted session');
  const old=await snapshot(app); expect(old.compatibleSettingsSHA256).toBe(before.compatibleSettingsSHA256);
  expect(old.stableDataSHA256).toEqual(before.stableDataSHA256);
  expect(old.ownerPasskeyCounters).toHaveLength(before.ownerPasskeyCounters.length);
  for(let index=0;index<old.ownerPasskeyCounters.length;index++) expect(old.ownerPasskeyCounters[index]).toBeGreaterThan(before.ownerPasskeyCounters[index]);
  await stop(app); app.legacy=false; start(app); await ready(app);
  expect((await api(app.owner,'/api/v1/settings')).body.publicSessionTimeoutsConfigured).toBe(false);
  await me(app,removed,401,'upgrade after rollback cannot resurrect session');
  await me(app,valid,401,'upgrade retains rollback session revocations');
  const upgraded=await snapshot(app);
  expect(upgraded.unrelatedDataSHA256).toEqual(old.unrelatedDataSHA256);
  expect(upgraded.compatibleSettingsSHA256).toBe(old.compatibleSettingsSHA256);
  await removed.ctx.close(); await valid.ctx.close(); record(app,'real downgrade and upgrade preserve unrelated data and current revocations');
}
return {configUI,legacyChecks,admissionChecks,atomicChecks,uiStates,enableMFAConcurrency,rollbackChecks};
}
