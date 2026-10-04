// Actual native fetch navigation, with real Server HTML. Only failure transport
// is injected; result-list changes after a requested search are expected.
export async function measureSubtitleSearch(page, viewport, results, probe) {
  probe.stage = "subtitle-native-search";
  const search = page.locator("#subtitle-search");
  if (!await search.isVisible()) throw new Error("Synthetic Library needs its native search");
  const geometry = () => page.evaluate(() => [".app-header", "#subtitle-list-title", ".subtitle-filters", "#subtitle-content"].map(selector => {
    const node = document.querySelector(selector), rect = node?.getBoundingClientRect();
    return {selector, present: Boolean(rect?.height), x: rect?.x, documentY: rect?.y + scrollY, width: rect?.width, height: rect?.height};
  }));
  const unchanged = (before, after) => before.every((first, index) => first.present && after[index]?.present &&
    ["x", "documentY", "width", "height"].every(key => Math.abs(first[key] - after[index][key]) <= 1));
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
    const before = await geometry();
    const retry = state === "retry-real-link";
    if (retry) await page.getByRole("link", {name: "Reload view", exact: true}).click();
    else await search.fill(query);
    await page.waitForFunction(() => document.querySelector("#main")?.getAttribute("aria-busy") === "true");
    await page.waitForTimeout(200);
    const pending = await geometry();
    await page.waitForFunction(() => document.querySelector("#main")?.getAttribute("aria-busy") !== "true");
    const after = await geometry(), errorVisible = await page.locator("#subtitle-feedback[data-error]").isVisible();
    results.push({flow: "subtitle-native-search", state, viewport, injectedFailure: injected, before, pending, after,
      pendingStable: unchanged(before, pending), failureRetainsContent: !injected || unchanged(before, after),
      errorVisible, stable: !injected || errorVisible, focusRetained: retry ? undefined : await search.evaluate(node => node === document.activeElement),
      overflow: await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)});
  }
}
