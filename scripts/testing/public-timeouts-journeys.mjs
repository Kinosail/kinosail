export function extraJourneys(deps) {
const {api,expect,record,fixture,signIn,me,save,stop,start,ready,spawnSync,settingsPath,join,evidence}=deps;
async function configUI(app) {
  const page=app.owner; app.lastpage=page;
  for(const width of [1440,1024,390,320]) {
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
return {configUI,legacyChecks,admissionChecks,atomicChecks};
}
