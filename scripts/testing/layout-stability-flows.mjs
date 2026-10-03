// Real HTMX bodies; only the explicitly labelled transport failure is injected.
export async function measureFlows(browser, options, watchPath) {
  const results = [];
  for (const viewport of [{width:390,height:844},{width:1440,height:900}]) {
    const context = await browser.newContext({...options,viewport,ignoreHTTPSErrors:false,reducedMotion:"reduce"});
    const page = await context.newPage();
    await page.goto("/?view=movies");
    const search = page.locator(".app-header input[name=q]");
    if (!await search.count()) {results.push({viewport,flow:"HTMX search",result:"not present"});await context.close();continue;}
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
    await context.close();
  }
  const context = await browser.newContext({...options,viewport:{width:390,height:844},ignoreHTTPSErrors:false,reducedMotion:"reduce"});
  const page = await context.newPage();
  await page.goto(watchPath);
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
  return results;
}
