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
    await summary.click();
    const open=await file.evaluate(node=>node.open),detail=await file.locator(".subtitle-file-detail").boundingBox(),overflow=await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth);
    results.push({flow:"subtitle-library-disclosure-open",viewport,open,detail,overflow,stable:Boolean(open&&detail&&detail.width&&overflow<=1)});
    await summary.click();if(await file.evaluate(node=>node.open))throw new Error("Library disclosure did not close");
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
    if (state !== "retry-real-link") await search.focus();
    else {const link=page.getByRole("link",{name:"Reload view",exact:true});await link.scrollIntoViewIfNeeded();await link.focus();}
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
    results.push({flow: "subtitle-native-search", state, viewport, injectedFailure: injected, before, pending, after,
      pendingStable: unchanged(before, pending), failureRetainsContent: !injected || (sameContentNode && unchanged(before, after)),
      errorVisible, sameContentNode, navigationSucceeded: injected ? undefined : navigationSucceeded, caretPreserved,
      stable: injected ? errorVisible : navigationSucceeded, focusRetained: retry ? undefined : await search.evaluate(node => node === document.activeElement),
      overflow: await page.evaluate(() => document.documentElement.scrollWidth - innerWidth),
      overflowNodes:await page.evaluate(()=>[...document.querySelectorAll("body *:not(option):not(optgroup)")].filter(node=>{const box=node.getBoundingClientRect();return box.height>0&&box.right+(getComputedStyle(node).position==="fixed"?0:scrollX)>innerWidth+1;}).slice(0,12).map(node=>({node:node.id||node.className||node.tagName,rect:node.getBoundingClientRect().toJSON(),minWidth:getComputedStyle(node).minWidth,whiteSpace:getComputedStyle(node).whiteSpace}))) });
  }
}
