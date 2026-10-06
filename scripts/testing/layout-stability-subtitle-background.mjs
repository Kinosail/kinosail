import {navigationDiagnostics} from "./navigation-diagnostics.mjs";
import {link, unlink} from "node:fs/promises";
import {join, resolve} from "node:path";

// Real fixture file discovery and the normal summary poll; no fake clock or API
// response. The current page must retain its snapshot until the user refreshes.
export async function measureSubtitleBackground(browser, options, results, probe) {
  const root = resolve(process.env.KINOSAIL_LAYOUT_MEDIA_ROOT || "");
  if (!root.includes("/.verification/layout/") || !root.endsWith("/media")) throw new Error("Owned synthetic media root required");
  const context = await browser.newContext({...options, viewport:{width:390,height:844},ignoreHTTPSErrors:false,reducedMotion:"reduce",serviceWorkers:"block"});
  const page = await context.newPage(), added = join(root,"Layout Background Example.mp4");
  const geometry = () => page.evaluate(() => ["#subtitle-content", "#subtitle-list-title", ".subtitle-filters", ".subtitle-file-list"].map(selector => {
    const box = document.querySelector(selector)?.getBoundingClientRect();
    return {selector,present:Boolean(box?.height),x:box?.x,documentY:box?.y+scrollY,width:box?.width,height:box?.height};
  }));
  const scan = () => page.evaluate(async () => {
    const token=document.querySelector('meta[name="kinosail-csrf"]')?.content;
    const controller=new AbortController(),timer=setTimeout(()=>controller.abort(),30_000);
    try {
      const response=await fetch("/scan",{method:"POST",headers:token?{"X-Kinosail-CSRF":token}:{},signal:controller.signal});
      return {ok:response.ok,status:response.status,authorizedLanding:new URL(response.url).pathname==="/"};
    } finally {clearTimeout(timer);}
  });
  let linked = false, countBefore = 0, phaseStarted = performance.now(), failed = false;
  const navigation = navigationDiagnostics(page, options.baseURL);
  const setPhase = phase => {
    phaseStarted = performance.now();
    probe.operationPhase = phase;
    probe.stage = "subtitle-background-" + phase;
  };
  const retainFailure = async error => {
    failed = true;
    probe.elapsedMs = Math.min(600000, Math.max(0, Math.round(performance.now() - phaseStarted)));
    probe.navigation = await navigation.snapshot(error);
  };
  setPhase("navigation");
  try {
    await page.goto("/?view=library",{waitUntil:"domcontentloaded"});
    setPhase("baseline-geometry");
    const before = await geometry(); countBefore = await page.locator(".subtitle-file").count();
    if(!countBefore)throw new Error("Populated synthetic Library required");
    setPhase("discovery-link");
    await link(join(root,"Layout Example.mp4"),added); linked = true;
    setPhase("scan-request");
    const discovery = await scan();
    results.push({flow:"subtitle-background-scan-request",browserOrigin:true,scanStatus:discovery.status,authorizedLanding:discovery.authorizedLanding,stable:discovery.ok&&discovery.authorizedLanding});
    if(!discovery.ok||!discovery.authorizedLanding)throw new Error("Synthetic discovery scan failed");
    setPhase("normal-poll");
    await page.locator("#subtitle-update").waitFor({state:"visible",timeout:75_000});
    setPhase("pending-geometry");
    const after = await geometry(), countAfter = await page.locator(".subtitle-file").count();
    const stable = countBefore>0 && countBefore===countAfter && before.every((first,index)=>first.present&&after[index]?.present&&["x","documentY","width","height"].every(key=>Math.abs(first[key]-after[index][key])<=1));
    results.push({flow:"subtitle-background-update",realFixtureDiscovery:true,normalPoll:true,scanStatus:discovery.status,countBefore,countAfter,before,after,noticeVisible:true,stable});
    setPhase("user-refresh");
    await page.locator("#subtitle-update [data-subtitle-refresh]").click();
    await page.waitForFunction(()=>document.querySelector("#main")?.getAttribute("aria-busy")!=="true");
    setPhase("refreshed-count");
    const refreshedCount = await page.locator(".subtitle-file").count();
    results.push({flow:"subtitle-background-user-refresh",refreshedCount,expectedCount:countBefore+1,stable:refreshedCount===countBefore+1});
  } catch (error) {
    await retainFailure(error);
    throw error;
  } finally {
    try {
      if(linked){
        const priorStage=probe.stage; if(!failed)setPhase("cleanup");
        await unlink(added);
        const cleanup=await scan(), authorized=cleanup.ok&&cleanup.authorizedLanding;
        if(authorized)await page.goto("/?view=library",{waitUntil:"domcontentloaded",timeout:20_000});
        const restoredCount=authorized?await page.locator(".subtitle-file").count():undefined;
        const stable=authorized&&restoredCount===countBefore;
        results.push({flow:"subtitle-background-cleanup",scanStatus:cleanup.status,authorizedLanding:cleanup.authorizedLanding,restoredCount,expectedCount:countBefore,stable});
        if(!stable)throw new Error("Synthetic catalogue restoration failed");
        probe.stage=priorStage;
      }
    } catch (error) {
      if (!failed) await retainFailure(error);
      throw error;
    } finally {
      navigation.stop();
      if (!failed) { delete probe.operationPhase; delete probe.elapsedMs; }
      await context.close();
    }
  }
}
