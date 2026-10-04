// Real HTMX bodies; only the explicitly labelled transport failure is injected.
import {measureSubtitleSearch} from "./layout-stability-subtitle-search.mjs";
import {measureSubtitleBackground} from "./layout-stability-subtitle-background.mjs";
export async function measureFlows(browser, options, watchPath, inspectorPath, results = [], probe = {}) {
  probe.stage = "HTMX-search";
  for (const viewport of [{width:390,height:844},{width:1440,height:900},...(inspectorPath?[{width:320,height:800}]:[])]) {
    probe.stage="library-journey-navigation";
    const context = await browser.newContext({...options,viewport,ignoreHTTPSErrors:false,reducedMotion:"reduce",serviceWorkers:"block"});
    const page = await context.newPage();
    if(inspectorPath&&viewport.width===320)await context.addInitScript(()=>{const apply=()=>{if(!document.documentElement)return false;document.documentElement.style.fontSize="200%";return true;};if(!apply()){const observer=new MutationObserver(()=>{if(apply())observer.disconnect();});observer.observe(document,{childList:true});}});
    await page.goto(inspectorPath ? "/?view=library" : "/?view=movies",{waitUntil:"domcontentloaded"});
    probe.stage="HTMX-search";
    if (inspectorPath) {await measureSubtitleSearch(page,viewport,results,probe);await context.close();continue;}
    const search = page.locator(".app-header input[name=q]");
    const searchPresent=Boolean(await search.count()), searchVisible=searchPresent&&await search.isVisible(), htmxLoaded=await page.evaluate(()=>typeof window.htmx!=="undefined");
    const htmxSearch=searchPresent&&await search.evaluate(n=>n.getAttribute("hx-get")==="/"&&n.getAttribute("hx-target")==="#main"&&Boolean(n.getAttribute("hx-trigger")));
    if (!searchVisible||!htmxLoaded||!htmxSearch) {results.push({viewport,flow:"HTMX search",result:"not active on this surface",searchPresent,searchVisible,htmxLoaded,htmxSearch});await context.close();continue;}
    let fail = false;
    await page.route("**/*",async route=>{
      const request=route.request(),url=new URL(request.url());
      if(url.pathname==="/"&&request.headers()["hx-request"]==="true"){
        if(fail){await new Promise(r=>setTimeout(r,900));await route.abort("failed");return;}
        const response=await route.fetch();await new Promise(r=>setTimeout(r,900));await route.fulfill({response});
      } else await route.continue();
    });
    for(const [name,query,injected] of [["pending-success","Layout",false],["pending-failure","Missing",true],["retry-empty","no-synthetic-match",false]]){
      fail=injected;
      const before=await page.locator("#main").boundingBox();
      await search.fill(query);
      await page.waitForFunction(()=>document.querySelector("#main")?.getAttribute("aria-busy")==="true");
      await page.waitForTimeout(200);
      const pending=await page.locator("#main").boundingBox();
      const busy=await page.locator("#main").evaluate(n=>({inert:n.inert,skeleton:n.classList.contains("request-skeleton")}));
      await page.waitForFunction(()=>document.querySelector("#main")?.getAttribute("aria-busy")!=="true");
      await page.waitForTimeout(200);
      const settled=await page.locator("#main").evaluate(n=>({inert:n.inert,skeleton:n.classList.contains("request-skeleton")}));
      results.push({viewport,flow:name,injectedFailure:injected,before,pending,busy,settled,
        pendingStable:JSON.stringify(before)===JSON.stringify(pending),focusRetained:await search.evaluate(n=>n===document.activeElement)});
    }
    await page.setViewportSize({width:844,height:390});
    await page.waitForTimeout(300);
    const resizeBefore=await page.locator("#main").boundingBox();
    await page.waitForTimeout(1200);
    const resizeAfter=await page.locator("#main").boundingBox();
    results.push({viewport,flow:"resize-after-settling",resizeBefore,resizeAfter,
      stable:JSON.stringify(resizeBefore)===JSON.stringify(resizeAfter),overflow:await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)});
    if(viewport.width===390){
      probe.stage="mobile-tabs-customize-focus";await page.setViewportSize(viewport);
      let mainHeld=false, releaseMain;
      const mainBarrier=new Promise(resolve=>{releaseMain=resolve;});
      await page.route("**/static/main.kinosail.bundle.js*",async route=>{mainHeld=true;try{const response=await route.fetch();await mainBarrier;await route.fulfill({response});}finally{mainHeld=false;}});
      try{
        await page.goto("/settings#navigation",{waitUntil:"commit"});
        for(const settings of [false,true]){
        const trigger=settings?page.locator("[data-customize-tabs]"):page.locator(".nav-more>summary");
        if(!settings){await trigger.click();await page.getByRole("button",{name:"Customize tabs",exact:true}).click();}else await trigger.click();
        await page.locator(".tab-editor").waitFor({state:"visible"});const pendingAtOpen=mainHeld;await page.keyboard.press("Escape");await page.locator(".tab-editor").waitFor({state:"hidden"});
        const focusRetained=await trigger.evaluate(n=>n===document.activeElement);results.push({viewport,flow:settings?"settings-customize-tabs-focus":"more-customize-tabs-focus",pendingAtOpen,serviceWorkers:"blocked for transport-controlled flow",focusRetained,stable:settings||pendingAtOpen});
        if(!settings){releaseMain();await page.waitForLoadState("domcontentloaded");}
        }
      }finally{releaseMain();}
    }
    await context.close();
  }
  if(inspectorPath)await measureSubtitleBackground(browser,options,results,probe);
  if(inspectorPath)for(const viewport of [{width:320,height:800},{width:390,height:844},{width:844,height:390}]){
    probe.stage="enlarged-dock-end-focus";
    const context=await browser.newContext({...options,viewport,ignoreHTTPSErrors:false,reducedMotion:"reduce"});
    await context.addInitScript(()=>{const apply=()=>{if(!document.documentElement)return false;document.documentElement.style.fontSize="200%";return true;};if(!apply()){const observer=new MutationObserver(()=>{if(apply())observer.disconnect();});observer.observe(document,{childList:true});}});
    const page=await context.newPage();await page.goto("/settings#thanks");
    const last=page.locator("main").locator('button:enabled, input:enabled:not([type=hidden]), select:enabled, textarea:enabled, a[href]').last();
    await last.focus();
    await page.keyboard.press("Shift+Tab");await page.keyboard.press("Tab");
    await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(resolve)));
    const control=await last.boundingBox(), dock=await page.locator("[data-subtitle-dock]").boundingBox(), header=await page.locator(".app-header").boundingBox();
    const focusRetained=await last.evaluate(n=>n===document.activeElement);
    await page.evaluate(()=>window.scrollTo({top:document.documentElement.scrollHeight,behavior:"instant"}));
    const tail=await page.locator("#thanks").boundingBox(), endDock=await page.locator("[data-subtitle-dock]").boundingBox();
    const reservation=await page.locator("main").evaluate(n=>({paddingBottom:getComputedStyle(n).paddingBottom,dockHeight:document.documentElement.style.getPropertyValue("--subtitle-dock-height"),scrollX,scrollY}));
    const overflow=await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth);
    results.push({flow:"enlarged-dock-end-focus",viewport,control,tail,dock,endDock,header,reservation,focusRetained,overflow,stable:Boolean(control&&tail&&dock&&endDock&&header&&control.y+control.height<=dock.y+1&&tail.y+tail.height<=endDock.y+1&&control.y>=header.y+header.height-1)});
    await context.close();
  }
  const context = await browser.newContext({...options,viewport:{width:390,height:844},ignoreHTTPSErrors:false,reducedMotion:"reduce"});
  probe.stage = "theater-idle-exit";
  const page = await context.newPage();
  await page.goto(watchPath,{waitUntil:"domcontentloaded"});
  probe.media = await page.locator("video").evaluate(video=>({readyState:video.readyState,errorCode:video.error?.code,mp4:video.canPlayType('video/mp4; codecs="avc1.42E01E"')}));
  const theater = page.locator("[data-theater]");
  if (await theater.isVisible()) {
    await page.locator("video").evaluate(video=>{video.loop=true;});
    if(await page.locator("video").evaluate(video=>video.paused))await page.getByRole("button",{name:"Play",exact:true}).first().click();
    await theater.click();
    await page.mouse.move(0,0);await page.waitForTimeout(2700);
    const hiddenAfterIdle=await page.locator(".player-stage-toolbar").evaluate(n=>n.hidden);
    await page.keyboard.press("Escape");await page.waitForTimeout(100);
    const visibleAfterExit=await page.locator(".player-stage-toolbar").isVisible();
    const before=await page.locator(".media-stage").boundingBox();
    const stage=await page.locator(".media-stage").boundingBox();await page.mouse.move(stage.x+15,stage.y+15);await page.waitForTimeout(200);
    const after=await page.locator(".media-stage").boundingBox();
    results.push({flow:"theater-idle-exit",hiddenAfterIdle,visibleAfterExit,before,after,stable:hiddenAfterIdle&&visibleAfterExit&&JSON.stringify(before)===JSON.stringify(after)});
  } else results.push({flow:"theater-idle-exit",result:"native control mode has no Theater"});
  await context.close();
  if(inspectorPath){
    probe.stage = "inspector-refresh-failure-retry";
    const context=await browser.newContext({...options,viewport:{width:390,height:844},ignoreHTTPSErrors:false,reducedMotion:"reduce"});
    const page=await context.newPage();
    await page.route("**/inspect?*",async route=>{if(route.request().resourceType()==="fetch"){await new Promise(r=>setTimeout(r,900));await route.abort("failed");}else await route.continue();});
    await page.goto(inspectorPath,{waitUntil:"commit"});await page.locator(".subtitle-inspector-workspace").waitFor({state:"visible"});
    const before=await page.locator(".subtitle-inspector-workspace").boundingBox();const pendingDisabled=await page.locator('#subtitle-edit-form button[type="submit"]').isDisabled();await page.waitForTimeout(2000);
    const after=await page.locator(".subtitle-inspector-workspace").boundingBox();
    const failedDisabled=await page.locator('#subtitle-edit-form button[type="submit"]').isDisabled();const qualityRows=await page.locator("#subtitle-quality p").count();const busy=await page.locator("#inspector-status").getAttribute("aria-busy");await page.unroute("**/inspect?*");await page.reload();await page.waitForFunction(()=>!document.querySelector('#subtitle-edit-form button[type="submit"]').disabled);
    results.push({flow:"inspector-refresh-failure-retry",injectedFailure:true,before,after,pendingDisabled,failedDisabled,busy,qualityRows,retryCompleted:true,stable:pendingDisabled&&failedDisabled&&!busy&&qualityRows>0&&JSON.stringify(before)===JSON.stringify(after)});
    await context.close();
    const editing=await browser.newContext({...options,viewport:{width:390,height:844},ignoreHTTPSErrors:false,reducedMotion:"reduce"});
    probe.stage = "inspector-edit-during-refresh";
    const editor=await editing.newPage();
    await editor.route("**/inspect?*",async route=>{if(route.request().resourceType()==="fetch"){const response=await route.fetch();await new Promise(r=>setTimeout(r,900));await route.fulfill({response});}else await route.continue();});
    await editor.goto(inspectorPath,{waitUntil:"commit"});
    await editor.waitForFunction(()=>document.querySelector("#inspector-status")?.getAttribute("aria-busy")==="true");
    const file=editor.locator('input[type="file"]');const pendingLocked=await file.isDisabled();
    const payload={name:"layout-input.srt",mimeType:"application/x-subrip",buffer:Buffer.from("1\n00:00:00,000 --> 00:00:01,000\nSynthetic edit.\n")};
    if(!pendingLocked)await file.setInputFiles(payload);
    await editor.waitForTimeout(1500);const preview=editor.locator('#subtitle-edit-form button[type="submit"]');
    const loadedEnabled=!await preview.isDisabled();
    if(pendingLocked&&loadedEnabled){await file.focus();await file.setInputFiles(payload);}
    results.push({flow:"inspector-edit-during-refresh",pendingLocked,loadedEnabled,stable:pendingLocked&&loadedEnabled});
    for(const mode of ["retain","tab-away","retain-scroll"]){
      probe.stage="inspector-language-refresh-focus";await editor.reload();await editor.waitForFunction(()=>!document.querySelector('#subtitle-edit-form button[type="submit"]').disabled);
      const language=editor.locator('select[name="language"]');const alternate=await language.evaluate((n,mode)=>[...n.options].find(option=>option.value&&option.value!==n.value&&(mode!=="retain-scroll"||option.value==="fr"))?.value,mode);
      if(!alternate)throw new Error("Synthetic inspector needs an alternate language");
      const originalScroll=await editor.evaluate(()=>scrollY),workspaceBefore=await editor.locator(".subtitle-inspector-workspace").boundingBox(),statusBefore=await editor.locator("#inspector-status").boundingBox();
      await language.focus();await language.selectOption(alternate);await editor.waitForFunction(()=>document.querySelector("#inspector-status")?.getAttribute("aria-busy")==="true");
      if(mode==="tab-away")await editor.keyboard.press("Tab");
      if(mode==="retain-scroll")await editor.evaluate(()=>scrollTo({top:Math.max(0,document.documentElement.scrollHeight-innerHeight-160),behavior:"instant"}));
      const selectorOffscreen=await language.evaluate(n=>{const r=n.getBoundingClientRect();return r.bottom<=0||r.top>=innerHeight;});
      const scrollBefore=await editor.evaluate(()=>scrollY),statusPending=await editor.locator("#inspector-status").boundingBox(),focus=await editor.evaluateHandle(()=>document.activeElement);await editor.waitForFunction(()=>document.querySelector("#inspector-status")?.getAttribute("aria-busy")!=="true");
      const focusRetained=mode==="tab-away"?await editor.evaluate(n=>n===document.activeElement,focus):await language.evaluate(n=>n===document.activeElement);await focus.dispose();
      const scrollAfter=await editor.evaluate(()=>scrollY),workspaceAfter=await editor.locator(".subtitle-inspector-workspace").boundingBox(),statusAfter=await editor.locator("#inspector-status").boundingBox();
      results.push({flow:mode==="retain-scroll"?"inspector-language-refresh-preserve-scroll":mode==="tab-away"?"inspector-language-refresh-tab-away":"inspector-language-refresh-focus",focusRetained,originalScroll,scrollBefore,scrollAfter,selectorOffscreen,workspaceBefore,workspaceAfter,statusBefore,statusPending,statusAfter,scrollRetained:mode!=="retain-scroll"||Boolean(Math.abs(scrollAfter-scrollBefore)<=1&&Math.abs(scrollBefore-originalScroll)>1&&selectorOffscreen&&workspaceBefore&&workspaceAfter&&Math.abs(workspaceBefore.height-workspaceAfter.height)<=1&&statusBefore&&statusPending&&statusAfter&&Math.abs(statusBefore.height-statusPending.height)<=1&&Math.abs(statusBefore.height-statusAfter.height)<=1)});
    }
    await editing.close();
  }
  return results;
}
