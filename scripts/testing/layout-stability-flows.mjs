// Real HTMX bodies; only the explicitly labelled transport failure is injected.
export async function measureFlows(browser, options) {
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
  return results;
}
