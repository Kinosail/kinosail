// Actual native fetch navigation, with real Server HTML. Only failure transport
// is injected; result-list changes after a requested search are expected.
export async function measureSubtitleSearch(page, viewport, results, probe) {
  probe.stage = "subtitle-native-search";
  const search = page.locator("#subtitle-search");
  if (!await search.isVisible()) throw new Error("Synthetic Library needs its native search");
  const geometry = () => page.evaluate(() => [".app-header", "#subtitle-list-title", ".subtitle-filters", "#subtitle-content"].map(selector => {
    const node = document.querySelector(selector), rect = node?.getBoundingClientRect();
    return {selector, present: Boolean(rect?.height), pinned: node && ["fixed", "sticky"].includes(getComputedStyle(node).position), x: rect?.x, y: rect?.y, documentY: rect?.y + scrollY, width: rect?.width, height: rect?.height};
  }));
  const unchanged = (before, after) => before.every((first, index) => first.present && after[index]?.present &&
    ["x", first.pinned ? "y" : "documentY", "width", "height"].every(key => Math.abs(first[key] - after[index][key]) <= 1));
  if(viewport.width===320){
    const file=page.locator(".subtitle-file").first(),summary=file.locator(":scope>summary");
    const clickSummary=async phase=>{
      probe.stage=phase;await summary.scrollIntoViewIfNeeded();
      const target=await summary.evaluate(node=>{
        const rect=node.getBoundingClientRect(),left=Math.max(0,rect.left),right=Math.min(innerWidth,rect.right),top=Math.max(0,rect.top),bottom=Math.min(innerHeight,rect.bottom);
        const exposed=(x,y)=>Boolean(node.contains(document.elementFromPoint(x,y)));
        let point;
        for(const fractionY of [.5,.4,.6,.3,.7,.2,.8,.1,.9])for(const fractionX of [.5,.3,.7]){
          const x=left+(right-left)*fractionX,y=top+(bottom-top)*fractionY;
          if(!point&&right>left&&bottom>top&&exposed(x,y))point={x:x-rect.x,y:y-rect.y};
        }
        return {rect:rect.toJSON(),point,centerExposed:exposed(rect.x+rect.width/2,rect.y+rect.height/2),header:document.querySelector(".app-header")?.getBoundingClientRect().toJSON(),dock:document.querySelector("[data-subtitle-dock]")?.getBoundingClientRect().toJSON(),viewport:{width:innerWidth,height:innerHeight}};
      });
      probe.geometry=target;if(!target.point)throw new Error("Library disclosure has no exposed pointer position");
      await summary.click({position:target.point});return target;
    };
    const interaction=await clickSummary("subtitle-library-disclosure-open");
    const open=await file.evaluate(node=>node.open),detail=await file.locator(".subtitle-file-detail").boundingBox(),overflow=await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth);
    results.push({flow:"subtitle-library-disclosure-open",viewport,open,detail,overflow,interaction,stable:Boolean(open&&detail&&detail.width&&overflow<=1)});
    await clickSummary("subtitle-library-disclosure-close");if(await file.evaluate(node=>node.open))throw new Error("Library disclosure did not close");
    probe.stage="subtitle-native-search";delete probe.geometry;
  }
  let fail = false;
  await page.route("**/*", async route => {
    const request = route.request(), url = new URL(request.url());
    if (url.pathname === "/" && request.resourceType() === "fetch") {
      if (fail) { await new Promise(resolve => setTimeout(resolve, 900)); await route.abort("failed"); return; }
      const response = await route.fetch(); await new Promise(resolve => setTimeout(resolve, 900)); await route.fulfill({response});
    } else await route.continue();
  });
  for (const [state, query, injected] of [["pending-success", "Layout", false], ["pending-failure", "Missing", true], ["retry-real-link", "Missing", false], ["retry-empty", "no-synthetic-match", false]]) {
    fail = injected;
    const control = state === "retry-real-link" ? page.getByRole("link", {name: "Reload view", exact: true}) : search;
    await control.scrollIntoViewIfNeeded();
    await control.focus();
    // Native focus scrolling must settle before measuring request-induced shifts.
    const focusBefore = await control.evaluate(node => new Promise((resolve, reject) => {
      let previous, stable = 0, finished = false, animation;
      const finish = (error, result) => {
        if (finished) return;
        finished = true; clearTimeout(timeout); cancelAnimationFrame(animation);
        if (error) reject(error); else resolve(result);
      };
      const timeout = setTimeout(() => finish(new Error("Focused search control did not settle")), 2000);
      const frame = () => {
        if (!node.isConnected) { finish(new Error("Focused search control was detached")); return; }
        if (document.querySelector("#main")?.getAttribute("aria-busy") === "true") {
          finish(new Error("Search request began before focus settled")); return;
        }
        const rect = node.getBoundingClientRect(), position = JSON.stringify([scrollX, scrollY, rect.x, rect.y, rect.width, rect.height]);
        stable = position === previous && node === document.activeElement && rect.bottom > 0 && rect.top < innerHeight ? stable + 1 : 0;
        previous = position;
        if (stable >= 3) finish(undefined, {scrollY, rect: rect.toJSON(), focused: true});
        else animation = requestAnimationFrame(frame);
      };
      animation = requestAnimationFrame(frame);
    }));
    const before = await geometry();
    const content = await page.locator("#subtitle-content").elementHandle();
    const retry = state === "retry-real-link";
    if (retry) await page.getByRole("link", {name: "Reload view", exact: true}).click();
    else await search.fill(query);
    await page.waitForFunction(() => document.querySelector("#main")?.getAttribute("aria-busy") === "true");
    await page.waitForTimeout(200);
    const pending = await geometry();
    await page.waitForFunction(() => document.querySelector("#main")?.getAttribute("aria-busy") !== "true");
    const after = await geometry(), errorVisible = await page.locator("#subtitle-feedback[data-error]").isVisible();
    const sameContentNode = await page.evaluate(node => node === document.getElementById("subtitle-content"), content);
    await content.dispose();
    const result = await page.evaluate(() => ({view: document.getElementById("main")?.dataset.view,
      query: new URL(location.href).searchParams.get("q"), items: document.querySelectorAll(".subtitle-file").length,
      empty: Boolean(document.querySelector(".subtitle-empty h3")?.textContent.includes("No files match."))}));
    const navigationSucceeded = !errorVisible && result.view === "library" && result.query === query &&
      (state === "pending-success" ? result.items > 0 : result.items === 0 && result.empty);
    const caretPreserved = retry ? undefined : await search.evaluate((node, length) => node.selectionStart === length && node.selectionEnd === length, query.length);
    results.push({flow: "subtitle-native-search", state, viewport, injectedFailure: injected, focusBefore, before, pending, after,
      pendingStable: unchanged(before, pending), failureRetainsContent: !injected || (sameContentNode && unchanged(before, after)),
      errorVisible, sameContentNode, navigationSucceeded: injected ? undefined : navigationSucceeded, caretPreserved,
      stable: injected ? errorVisible : navigationSucceeded, focusRetained: retry ? undefined : await search.evaluate(node => node === document.activeElement),
      overflow: await page.evaluate(() => document.documentElement.scrollWidth - innerWidth),
      overflowNodes:await page.evaluate(()=>[...document.querySelectorAll("body *:not(option):not(optgroup)")].filter(node=>{const box=node.getBoundingClientRect();return box.height>0&&box.right+(getComputedStyle(node).position==="fixed"?0:scrollX)>innerWidth+1;}).slice(0,12).map(node=>({node:node.id||node.className||node.tagName,rect:node.getBoundingClientRect().toJSON(),minWidth:getComputedStyle(node).minWidth,whiteSpace:getComputedStyle(node).whiteSpace}))) });
  }
}
