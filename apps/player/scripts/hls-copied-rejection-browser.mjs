import {createRequire} from 'node:module';
import {join} from 'node:path';
import {writeFile} from 'node:fs/promises';
const root=process.cwd(), require=createRequire(join(root,'apps/subtitles/e2e/package.json'));
const {chromium,webkit}=require('@playwright/test');
const origin=process.env.QA_RESUME_ORIGIN, token=process.env.QA_RESUME_TOKEN, id=process.env.QA_RESUME_ITEM;
const output=process.env.QA_RESUME_OUTPUT, engine=process.env.QA_RESUME_ENGINE;
const channel=process.env.PLAYWRIGHT_CHANNEL || (!process.env.CI && process.platform==='darwin'?'chrome':undefined);
const browser=await (engine==='webkit'?webkit.launch():chromium.launch({channel}));
const cases=[];
let context;
// Generated FFmpeg inputs cannot represent malformed initialization boxes.
// This isolated network fault checks terminal rejection in the production UI.
let passed=true, stage='setup';
try {
 for(const viewport of [{width:390,height:844},{width:1440,height:900},{width:1920,height:1080}]) {
  context=await browser.newContext({serviceWorkers:'block',extraHTTPHeaders:{Authorization:`Bearer ${token}`}});
  const page=await context.newPage(); await page.setViewportSize(viewport);
  let release, initializationRequests=0;
  const held=new Promise(resolve=>release=resolve);
  await page.route(url=>url.origin===origin&&url.pathname.startsWith('/hls/')&&url.pathname.endsWith('/init.mp4'),async route=>{
   initializationRequests++;
   await held;
   const response=await route.fetch(), bytes=await response.body();
   const at=bytes.indexOf(Buffer.from('elst'));
   if(at<0)throw new Error('fixture_missing_edit');
   bytes.writeUInt32BE(0,at+8);
   await route.fulfill({response,body:bytes});
  });
  stage='navigation';
  await page.goto(`${origin}/watch/${id}?compatible=1`,{waitUntil:'domcontentloaded'});
  stage='pending';
  await page.waitForFunction(()=>document.querySelector('[data-playback-mode-status]')?.textContent.startsWith('Starting'));
  await page.screenshot({path:output.replace('.json',`-${viewport.width}-pending.png`)});
  stage='rejection'; release();
  await page.waitForFunction(()=>document.querySelector('[data-player-message]')?.textContent==='Compatibility playback is unavailable for this file.');
  stage='late-readiness';
  await page.locator('video').dispatchEvent('canplay');
  await page.locator('video').dispatchEvent('playing');
  await page.waitForTimeout(1000);
  const state=await page.evaluate(()=>{
   const status=document.querySelector('[data-player-status]'), skeleton=status.querySelector('.buffer-skeleton');
   return {label:document.querySelector('[data-playback-mode-status]').textContent,
    message:document.querySelector('[data-player-message]').textContent,state:status.dataset.state,
    busy:status.getAttribute('aria-busy'),visible:!status.hidden,
    skeletonHidden:!skeleton||getComputedStyle(skeleton).display==='none',paused:document.querySelector('video').paused};
  });
  const ok=state.label==='Compatibility stopped'&&state.state==='error'&&state.busy==='false'&&state.visible&&state.skeletonHidden&&state.paused&&initializationRequests===1;
  passed&&=ok; cases.push({viewport,...state,initializationRequests,passed:ok});
  await page.screenshot({path:output.replace('.json',`-${viewport.width}-failed.png`)});
  await context.close();
 }
} catch(error) {passed=false;cases.push({stage,failureClass:error.name,probe:await context.pages().at(-1)?.evaluate(()=>{const video=document.querySelector('video');return {mse:video?.currentSrc.startsWith('blob:'),readyState:video?.readyState,errorCode:video?.error?.code,label:document.querySelector('[data-playback-mode-status]')?.textContent,message:document.querySelector('[data-player-message]')?.textContent,hlsLoaded:typeof Hls!=='undefined',hlsSupported:typeof Hls!=='undefined'&&Hls.isSupported()};}).catch(()=>null)});}
finally {await context?.close();await browser.close();}
await writeFile(output,JSON.stringify({engine,passed,proofClass:'Isolated malformed-init network injection, fresh browser contexts, service workers blocked; real authenticated Server and production page',cases},null,2)+'\n');
console.log(JSON.stringify({engine,passed}));process.exitCode=passed?0:1;
