import type {Page, Response, TestInfo} from "@playwright/test";

// This recovery is confined to the disposable pagination fixture. Preserve the
// first real catalog before releasing Firefox's stranded navigation with one GET.
export async function gotoPaginationCatalog(page: Page, url: string, info: TestInfo) {
  const previousDocument = await page.evaluate(() => performance.timeOrigin);
  const started = Date.now();
  let observed: Response | undefined, bootstrap: Response | undefined, responseAfter = Infinity, documents = 0;
  const observe = (response: Response) => {
    const request = response.request();
    const address = new URL(response.url());
    if (request.resourceType() === "script" && address.origin === new URL(url).origin && address.pathname === "/static/main.kinosail.bundle.js") bootstrap = response;
    if (request.isNavigationRequest() && request.frame() === page.mainFrame() && request.method() === "GET" && response.url() === url) {
      observed = response;
      responseAfter = Date.now() - started;
      documents++;
    }
  };
  const navigate = () => page.goto(url, {waitUntil: "commit", timeout: 10_000});
  page.on("response", observe);
  try {
    return await navigate();
  } catch (error) {
    if (page.context().browser()?.browserType().name() !== "firefox" || !(error instanceof Error) || error.name !== "TimeoutError" ||
        documents !== 1 || !observed || responseAfter > 2000 || observed.status() !== 200 ||
        observed.request().redirectedFrom() || observed.request().redirectedTo() || !bootstrap || bootstrap.status() !== 200 ||
        bootstrap.request().redirectedFrom() || bootstrap.request().redirectedTo()) throw error;
    const originalDocument = await page.evaluate(expectedURL => {
      const expected = new URL(expectedURL), showsOnly = expected.search === "?view=shows&limit=4";
      const supported = showsOnly || expected.search === "?q=Pagination&limit=4";
      const total = showsOnly ? 30 : 36;
      const search = document.querySelector<HTMLInputElement>("#library-search");
      const library = document.querySelector<HTMLElement>("#library");
      const status = document.querySelector<HTMLElement>("[data-library-status]");
      const navigation = document.querySelector("[data-library-pagination]");
      const next = document.querySelector<HTMLAnchorElement>("[data-library-next]");
      const visible = (element: HTMLElement | null) => Boolean(element && !element.closest("[inert]") &&
        element.getBoundingClientRect().width && element.getBoundingClientRect().height && getComputedStyle(element).visibility === "visible");
      const cards = [...document.querySelectorAll<HTMLElement>("#library .card")].map(card => {
        const link = card.matches("a[href]") ? card as HTMLAnchorElement : card.querySelector<HTMLAnchorElement>("a.show-details");
        return {title: card.querySelector("h2")?.textContent?.trim(), href: link?.getAttribute("href"),
          group: card.closest<HTMLElement>("[data-library-group]")?.dataset.libraryGroup, visible: visible(link)};
      });
      const validCards = cards.length >= 4 && cards.length <= total && new Set(cards.map(card => card.href)).size === cards.length &&
        new Set(cards.map(card => card.title)).size === cards.length && cards.every(card => card.visible && (
          card.group === "shows" && /^Pagination Show (?:0[1-9]|[12][0-9]|30)$/.test(card.title || "") && /^\/show\/[a-f0-9]{16}$/.test(card.href || "") ||
          !showsOnly && card.group === "movies" && /^Pagination Movie 0[1-6]$/.test(card.title || "") && /^\/watch\/[a-f0-9]{16}$/.test(card.href || "")));
      const nextURL = next && new URL(next.href);
      const offset = nextURL?.searchParams.get("offset") || "";
      const validNext = nextURL ? nextURL.origin === expected.origin && nextURL.pathname === "/" && !nextURL.hash &&
        next.hidden && nextURL.searchParams.get("limit") === "4" && /^(?:[1-9][0-9]*)$/.test(offset) && Number(offset) === cards.length && Number(offset) < total &&
        Number(offset) % 4 === 0 && nextURL.searchParams.get("q") === (showsOnly ? null : "Pagination") &&
        nextURL.searchParams.get("view") === (showsOnly ? "shows" : null) &&
        [...nextURL.searchParams].length === 3 && new Set(nextURL.searchParams.keys()).size === 3 :
        cards.length === total && status?.textContent === "All titles are loaded.";
      return {url: location.href, timeOrigin: performance.timeOrigin, state: document.readyState, cards,
        bootstrap: document.querySelector<HTMLScriptElement>('script[src^="/static/main.kinosail.bundle.js?"]')?.src,
        next: next?.getAttribute("href"), status: status?.textContent, failure: status?.dataset.failure,
        ready: Boolean(supported && visible(library) && visible(search) && !search?.disabled && !search?.readOnly &&
          search?.value === (showsOnly ? "" : "Pagination") && validCards && status && !status.dataset.failure && validNext &&
          navigation?.getAttribute("aria-busy") !== "true" && !next?.dataset.loading &&
          /^(?:|[0-9]+ more titles loaded\.|All titles are loaded\.)$/.test(status.textContent || ""))};
    }, url);
    if (!originalDocument.ready || originalDocument.bootstrap !== bootstrap.url() || originalDocument.state !== "complete" || originalDocument.timeOrigin === previousDocument ||
        originalDocument.url !== url) throw error;
    page.off("response", observe);
    await info.attach("firefox-catalog-navigation-recovery", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      status: observed.status(), bootstrapStatus: bootstrap.status(), responseAfter, elapsed: Date.now() - started, originalDocument,
      browser: page.context().browser()?.version(), action: "one superseding GET after verified original catalog"}), contentType: "application/json"});
    return await navigate();
  } finally {
    page.off("response", observe);
  }
}
