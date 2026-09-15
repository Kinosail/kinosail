const {createRequire}=require('node:module');
const path=require('node:path');
const {pathToFileURL}=require('node:url');
const requireApp=createRequire(path.resolve(__dirname,'../../apps/dashboard/e2e/package.json'));
const {chromium,firefox,webkit}=requireApp('@playwright/test');
const AxeBuilder=requireApp('@axe-core/playwright').default;
const assert=require('node:assert/strict');
const fs=require('node:fs');
const url=pathToFileURL(path.join(__dirname,'modern-family-ui.html')).href;
const output=fs.mkdtempSync(path.join(require('node:os').tmpdir(),'kinosail-design-reference-'));
console.log('Rendered evidence: '+output);
(async()=>{
for (const [name,engine] of Object.entries({chromium,firefox,webkit})) {
const browser=await engine.launch(); console.log(name+': '+browser.version()); const context=await browser.newContext(); const page=await context.newPage();
const errors=[];page.on('pageerror',e=>errors.push(e.message));
for (const width of [1440,1024,720,390,320]) {
await page.setViewportSize({width,height:900});await page.goto(url);
for (const theme of ['dark','light']) {
if(theme==='light')await page.locator('#theme').click();
await page.waitForTimeout(250);
assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),`${name} ${width} ${theme} overflow`);
const axe=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21aa','wcag22aa']).analyze();
assert.deepEqual(axe.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)})),[],`${name} ${width} ${theme} axe`);
await page.screenshot({path:`${output}/${name}-${width}-${theme}.png`,fullPage:true});
}
}
await page.setViewportSize({width:320,height:844}); await page.goto(url);
await page.emulateMedia({reducedMotion:'reduce'});await page.getByRole('link',{name:'Design lab',exact:true}).click();
assert((await page.locator('#lab-title').boundingBox()).y>=(await page.locator('.top').boundingBox()).height,`${name} anchor heading obscured`);
await page.emulateMedia({reducedMotion:'no-preference'});
await page.setViewportSize({width:390,height:844}); await page.goto(url);
await page.locator('input[value=services]').check();assert(await page.locator('#scene-services').isVisible());
await page.locator('input[value=operations]').check();await page.getByRole('button',{name:'Retry connection'}).click();assert.match(await page.locator('#preview-status').innerText(),/No provider/);
await page.locator('input[value=media]').check();await page.locator('#play-options').click();assert(await page.locator('dialog').isVisible());await page.keyboard.press('Escape');assert(!await page.locator('dialog').isVisible());assert(await page.locator('#play-options').evaluate(e=>e===document.activeElement));
await page.locator('input[value=compact]').check();assert.equal(await page.locator('#preview').getAttribute('data-density'),'compact');
await page.locator('[data-scenario=saved]').click();await page.getByLabel('French',{exact:true}).check();assert.match(await page.locator('#control-status').innerText(),/Unsaved/);await page.getByRole('button',{name:'Save preferences'}).click();assert.match(await page.locator('#control-status').innerText(),/English, French/);
await page.locator('[data-scenario=immediate]').click();await page.locator('#autoplay').check();assert.match(await page.locator('#control-status').innerText(),/is on/);
await page.locator('[data-scenario=long]').click();await page.locator('#language-search').fill('fren');await page.locator('[data-language=French]').click();await page.locator('#language-search').fill('zzzzz');assert(await page.locator('#language-empty').isVisible());assert.match(await page.locator('#language-selection').innerText(),/French/);
await page.locator('#language-search').fill('');assert.equal(await page.locator('[data-language]:visible').count(),6);
await page.locator('[data-scenario=native]').click();await page.locator('#track').selectOption({label:'Japanese · Audio description'});assert.match(await page.locator('#control-status').innerText(),/Japanese/);
await page.locator('[data-scenario=short]').click();await page.locator('input[value=Posters]').focus();await page.keyboard.press('ArrowRight');assert(await page.locator('input[value=List]').isChecked());
await page.emulateMedia({reducedMotion:'reduce',forcedColors:'active'});await page.goto(url);assert(await page.locator('#motion').isChecked());
await page.emulateMedia({reducedMotion:'no-preference'});assert.equal(await page.locator('.connected span').first().evaluate(e=>getComputedStyle(e).transitionDuration),'0s');
await page.locator('#motion').uncheck();assert.notEqual(await page.locator('.connected span').first().evaluate(e=>getComputedStyle(e).transitionDuration),'0s');
await page.emulateMedia({reducedMotion:'reduce'});assert.equal(await page.locator('.connected span').first().evaluate(e=>getComputedStyle(e).transitionDuration),'0s');await page.screenshot({path:`${output}/${name}-forced-colors.png`,fullPage:true});
assert.deepEqual(errors,[]);console.log(`${name}: 10 responsive/theme axe scans and all interactions passed`);await browser.close();
}
})().catch(e=>{console.error(e);process.exit(1)});
