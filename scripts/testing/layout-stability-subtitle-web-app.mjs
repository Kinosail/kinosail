import {createHash} from "node:crypto";
export async function measureSubtitleWebAppMetadata(browser, options, inspectorPath, results, probe) {
  const context = await browser.newContext({...options, viewport: {width: 390, height: 844}, ignoreHTTPSErrors: false});
  let page;
  try {
    page = await probe.openPage(context);
    const response = await context.request.get("/manifest.webmanifest");
    const bytes = await response.body();
    let manifest;
    try { manifest = JSON.parse(bytes.toString()); } catch { manifest = null; }
    const expected = {id: "/", name: "Kinosail Subtitles", short_name: "Subtitles", scope: "/", start_url: "/", display: "standalone"};
    const matches = value => Boolean(value && Object.entries(expected).every(([key, expected]) => value[key] === expected));
    const manifestValid = response.status() === 200 &&
      response.headers()["content-type"]?.split(";")[0] === "application/manifest+json" && matches(manifest);
    const manifestSHA256 = createHash("sha256").update(bytes).digest("hex");
    let failed = false;
    const cases = [
      ...["summary", "wanted", "library", "history"].map(view => ({name: view, path: `/?view=${view}`, view, selector: "#subtitle-content"})),
      {name: "inspector", path: inspectorPath, selector: "main.subtitle-inspector"},
      {name: "settings", path: "/settings", selector: ".settings-shell"},
      {name: "login-control", path: "/login", selector: "body.auth", anonymous: true}
    ];
    for (const testCase of cases) {
      probe.stage = "subtitle-dashboard-web-app-metadata";
      if (testCase.anonymous) await context.clearCookies();
      const navigation = await page.goto(testCase.path, {waitUntil: "domcontentloaded"});
      await page.locator(testCase.selector).waitFor({state: "attached"});
      const document = await page.evaluate(() => {
        const links = [...window.document.head.querySelectorAll('link[rel~="manifest"]')];
        return {view: window.document.querySelector("main")?.dataset.view,
          dockPresent: Boolean(window.document.querySelector("[data-subtitle-dock]")),
          manifestLinks: links.length,
          expectedManifestLinks: links.filter(link => {
            try { const url = new URL(link.href); return url.origin === location.origin && url.pathname === "/manifest.webmanifest"; }
            catch { return false; }
          }).length};
      });
      let processedManifest;
      if (browser.browserType().name() === "chromium") {
        const cdp = await context.newCDPSession(page);
        try {
          const processed = await cdp.send("Page.getAppManifest");
          let decoded, locationValid = false;
          try {
            decoded = JSON.parse(processed.data || "null");
            const url = new URL(processed.url);
            locationValid = url.origin === new URL(options.baseURL).origin && url.pathname === "/manifest.webmanifest";
          } catch {   }
          processedManifest = {locationValid, identityValid: matches(decoded), errors: processed.errors.length};
        } finally { await cdp.detach(); }
      }
      const stable = navigation?.status() === 200 && (!testCase.view || document.view === testCase.view) && document.dockPresent === (testCase.name !== "login-control") &&
        document.manifestLinks === 1 && document.expectedManifestLinks === 1 && manifestValid &&
        (!processedManifest || processedManifest.locationValid && processedManifest.identityValid && processedManifest.errors === 0);
      results.push({flow: probe.stage, page: testCase.name, document, manifestStatus: response.status(), manifestValid, manifestSHA256,
        processedManifest, stable, boundary: "Document and browser manifest metadata; no OS installation proof"});
      await probe.captureSnapshot(`subtitles-web-app-${testCase.name}`, page);
      failed ||= !stable;
    }
    if (failed) throw new Error("Subtitles production dashboard does not expose its web-app manifest");
  } catch (error) {
    await probe.captureFailure(error, page);
    throw error;
  } finally { await context.close(); }
}
