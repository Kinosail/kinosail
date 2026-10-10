import {expect, test, type Page, type Response} from "@playwright/test";
import {gotoPaginationCatalog} from "./library-navigation";

export function registerLibraryNavigationControls(origin: string | undefined) {
  for (const scenario of ["ready", "ready-shows", "missing-catalog", "missing-bootstrap", "wrong-query", "wrong-url", "invalid-card", "invalid-next", "skipped-offset", "busy", "failed", "pending-resource", "http-error", "redirect", "slow-response", "other-error", "repeated-timeout", "multiple-documents", "unchanged-document"] as const) {
    test(`catalog navigation recovery preserves original evidence: ${scenario}`, async ({page, browser}, info) => {
      // Inject faults after automatic paging settles beyond its 600px observer margin.
      await page.setViewportSize({width: 390, height: 844});
      const target = `${origin}/${scenario === "ready-shows" ? "?view=shows&limit=4" : "?q=Pagination&limit=4"}`;
      let gets = 0, posts = 0, calls = 0, redirects = 0, requested = false, release!: () => void;
      const held = new Promise<void>(resolve => {release = resolve;});
      const previousDocument = await page.evaluate(() => performance.timeOrigin);
      page.on("request", request => {
        if (request.isNavigationRequest() && request.frame() === page.mainFrame()) {
          if (request.method() === "GET") gets++;
          else posts++;
        }
      });
      page.on("response", response => {
        if (response.request().isNavigationRequest() && response.request().frame() === page.mainFrame() &&
            response.status() >= 300 && response.status() < 400) redirects++;
      });
      if (["http-error", "redirect", "slow-response"].includes(scenario)) await page.route(target, async route => {
        if (scenario === "http-error") return route.fulfill({status: 500, body: "Unavailable"});
        // WebKit cannot fulfill synthetic redirects. The Go Server canonicalizes
        // this real HTTP path, preserving an actual redirect response and chain.
        if (scenario === "redirect") return route.continue({url: `${origin}//?view=shows&limit=4`});
        await new Promise(resolve => setTimeout(resolve, 2100));
        return route.continue();
      });
      if (scenario === "missing-bootstrap") await page.route("**/static/main.kinosail.bundle.js?*", route => route.fulfill({status: 404, body: ""}));
      if (scenario === "pending-resource") {
        await page.route("**/qa-held-catalog-image.png", async route => {
          requested = true;
          await held;
          await route.fulfill({status: 204});
        });
        await page.addInitScript(() => document.addEventListener("DOMContentLoaded", () => {
          const image = new Image();
          image.src = "/qa-held-catalog-image.png";
          document.body.append(image);
        }, {once: true}));
      }
      const documentResponses: Response[] = [];
      let setupTimeouts = 0, injections = 0;
      page.on("response", response => {
        if (response.url() === target && response.request().isNavigationRequest() && response.request().frame() === page.mainFrame()) documentResponses.push(response);
      });
      const original = page.goto.bind(page);
      page.goto = async (...args: Parameters<Page["goto"]>) => {
        calls++;
        let response: Response | null = null;
        const previousResponses = documentResponses.length;
        try {
          try { response = await original(...args); }
          catch (error) {
            const documentResponse = documentResponses.at(-1);
            // A real Firefox bookkeeping timeout must not bypass fault injection.
            // Require the original successful document before preparing the control.
            if (browser.browserType().name() !== "firefox" || !(error instanceof Error) || error.name !== "TimeoutError" ||
                documentResponses.length !== previousResponses + 1 || !documentResponse || documentResponse.status() !== 200 || documentResponse.request().redirectedFrom() ||
                documentResponse.request().redirectedTo() || !await page.evaluate(({target, previousDocument}) =>
                  location.href === target && document.readyState === "complete" && performance.timeOrigin !== previousDocument,
                {target, previousDocument})) throw error;
            response = documentResponse;
            setupTimeouts++;
          }
          if (scenario === "pending-resource") await expect.poll(() => requested).toBe(true);
          else await page.waitForFunction(() => document.readyState === "complete");
          if (!["missing-bootstrap", "pending-resource", "http-error", "redirect"].includes(scenario)) {
            await page.waitForFunction(() => {
              const next = document.querySelector<HTMLAnchorElement>("[data-library-next]");
              const status = document.querySelector<HTMLElement>("[data-library-status]");
              return status && !status.dataset.failure && document.querySelector("[data-library-pagination]")?.getAttribute("aria-busy") !== "true" &&
                !next?.dataset.loading && (next ? next.hidden && status.getBoundingClientRect().top > innerHeight + 600 : status.textContent === "All titles are loaded.");
            }, undefined, {timeout: 10_000});
          }
        } catch (error) {
          throw new Error("Catalog navigation control setup failed", {cause: error});
        }
        if (calls === 1 || scenario === "repeated-timeout") {
          if (scenario === "multiple-documents") await original(...args);
          await page.evaluate(({scenario, previousDocument}) => {
            if (scenario === "missing-catalog") document.querySelector("#library")?.remove();
            if (scenario === "wrong-query") document.querySelector<HTMLInputElement>("#library-search")!.value = "Different";
            if (scenario === "wrong-url") history.replaceState(null, "", "/?q=Different&limit=4");
            if (scenario === "invalid-card") document.querySelector("#library .card h2")!.textContent = "Unexpected title";
            if (scenario === "invalid-next") document.querySelector("[data-library-next]")?.setAttribute("href", "https://invalid.example/");
            if (scenario === "skipped-offset") {
              const next = document.querySelector<HTMLAnchorElement>("[data-library-next]")!;
              const url = new URL(next.href);
              url.searchParams.set("offset", String(Number(url.searchParams.get("offset")) + 4));
              next.href = url.href;
            }
            if (scenario === "busy") document.querySelector("[data-library-pagination]")?.setAttribute("aria-busy", "true");
            if (scenario === "failed") document.querySelector<HTMLElement>("[data-library-status]")!.dataset.failure = "http";
            if (scenario === "unchanged-document") Object.defineProperty(performance, "timeOrigin", {value: previousDocument});
          }, {scenario, previousDocument});
          injections++;
          const error = new Error("Controlled navigation bookkeeping failure after real Server HTTP");
          error.name = scenario === "other-error" ? "Error" : "TimeoutError";
          throw error;
        }
        return response;
      };
      try {
        const result = await gotoPaginationCatalog(page, target, info).then(response => ({ok: true, status: response?.status()}), error => ({ok: false, error: String(error)}));
        const eligible = browser.browserType().name() === "firefox" && ["ready", "ready-shows"].includes(scenario);
        expect(injections).toBe(1 + Number(scenario === "repeated-timeout" && browser.browserType().name() === "firefox"));
        expect(result.ok).toBe(eligible);
        const recoveries = browser.browserType().name() === "firefox" && ["ready", "ready-shows", "repeated-timeout"].includes(scenario) ? 1 : 0;
        expect(calls).toBe(1 + recoveries);
        expect(gets).toBe(1 + recoveries + Number(scenario === "redirect" || scenario === "multiple-documents"));
        expect(posts).toBe(0);
        expect(redirects).toBe(Number(scenario === "redirect"));
        if (eligible) {
          expect(result.status).toBe(200);
          await expect(page.locator("#library .card").first()).toBeVisible();
          await expect(page.locator("#library-search")).toHaveValue(scenario === "ready-shows" ? "" : "Pagination");
        }
        await info.attach("catalog-navigation-control", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
          scenario, browser: browser.browserType().name(), version: browser.version(), calls, gets, posts, redirects, setupTimeouts, injections, result,
          data: "Disposable real Go Server catalog; injected automation timeout and documented original-document faults", outcome: "passed"}), contentType: "application/json"});
      } finally {
        release();
        await page.unrouteAll({behavior: "wait"});
      }
    });
  }
}
