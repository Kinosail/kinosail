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
  let linked = false, countBefore = 0, headers;
  probe.stage = "subtitle-background-discovery";
  try {
    await page.goto("/?view=library",{waitUntil:"domcontentloaded"});
    const before = await geometry(); countBefore = await page.locator(".subtitle-file").count();
    headers = {"X-Kinosail-CSRF":await page.locator('meta[name="kinosail-csrf"]').getAttribute("content")||""};
    if(!countBefore)throw new Error("Populated synthetic Library required");
    await link(join(root,"Layout Example.mp4"),added); linked = true;
    const scan = await page.request.post("/scan",{headers});
    if(!scan.ok())throw new Error("Synthetic discovery scan failed");
    probe.stage = "subtitle-background-normal-poll";
    await page.locator("#subtitle-update").waitFor({state:"visible",timeout:75_000});
    const after = await geometry(), countAfter = await page.locator(".subtitle-file").count();
    const stable = countBefore>0 && countBefore===countAfter && before.every((first,index)=>first.present&&after[index]?.present&&["x","documentY","width","height"].every(key=>Math.abs(first[key]-after[index][key])<=1));
    results.push({flow:"subtitle-background-update",realFixtureDiscovery:true,normalPoll:true,scanStatus:scan.status(),countBefore,countAfter,before,after,noticeVisible:true,stable});
    await page.locator("#subtitle-update [data-subtitle-refresh]").click();
    await page.waitForFunction(()=>document.querySelector("#main")?.getAttribute("aria-busy")!=="true");
    const refreshedCount = await page.locator(".subtitle-file").count();
    results.push({flow:"subtitle-background-user-refresh",refreshedCount,expectedCount:countBefore+1,stable:refreshedCount===countBefore+1});
  } finally {
    try {
      if(linked){
        const priorStage=probe.stage; probe.stage="subtitle-background-cleanup";
        await unlink(added);
        const scan=await page.request.post("/scan",{headers});
        if(scan.ok())await page.goto("/?view=library",{waitUntil:"domcontentloaded",timeout:20_000});
        const restoredCount=scan.ok()?await page.locator(".subtitle-file").count():undefined;
        const stable=scan.ok()&&restoredCount===countBefore;
        results.push({flow:"subtitle-background-cleanup",scanStatus:scan.status(),restoredCount,expectedCount:countBefore,stable});
        if(!stable)throw new Error("Synthetic catalogue restoration failed");
        probe.stage=priorStage;
      }
    } finally {await context.close();}
  }
}
