import type {Page, Response, TestInfo} from "@playwright/test";

// Firefox can commit a usable document while its automation navigation remains pending.
// A second GET is permitted only after preserving proof that the first form works.
export async function gotoAuthForm(page: Page, path: "/login" | "/login?next=/" | "/setup", info: TestInfo) {
  const previousDocument = await page.evaluate(() => performance.timeOrigin);
  const started = Date.now();
  let observed: Response | undefined, responseAfter = Infinity, documents = 0;
  const observe = (response: Response) => {
    const request = response.request(), url = new URL(response.url());
    if (request.isNavigationRequest() && request.frame() === page.mainFrame() && request.method() === "GET" &&
        url.pathname + url.search + url.hash === path) {
      observed = response;
      responseAfter = Date.now() - started;
      documents++;
    }
  };
  const navigate = async () => {
    const response = await page.goto(path, {waitUntil: "commit", timeout: 10_000});
    // Setup immediately branches on the rendered heading; preserve its loaded-document contract.
    if (path === "/setup") await page.waitForLoadState("load", {timeout: 10_000});
    return response;
  };
  page.on("response", observe);
  try {
    return await navigate();
  } catch (error) {
    if (page.context().browser()?.browserType().name() !== "firefox" || !(error instanceof Error) ||
        error.name !== "TimeoutError" || documents !== 1 || !observed || responseAfter > 2000 ||
        observed.status() !== 200 || observed.request().redirectedFrom()) throw error;
    const originalDocument = await page.evaluate(expectedPath => {
      const name = document.querySelector<HTMLInputElement>('input[name="name"]');
      const password = document.querySelector<HTMLInputElement>('input[name="password"][type="password"]');
      const form = password?.form;
      const toggle = password?.closest(".password-control")?.querySelector<HTMLButtonElement>('button.password-toggle[aria-label="Show secret"]');
      const submitLabel = expectedPath === "/setup" ? "Create Owner & continue" : "Sign in";
      const submit = form && [...form.querySelectorAll<HTMLButtonElement>("button")].find(button => button.type === "submit" && button.textContent?.trim() === submitLabel);
      const visible = (element: HTMLElement | null | undefined) => Boolean(element && !element.closest("[inert]") &&
        element.getBoundingClientRect().width && element.getBoundingClientRect().height && getComputedStyle(element).visibility === "visible");
      const editable = (input: HTMLInputElement | null) => Boolean(input && visible(input) && !input.matches(":disabled") && !input.readOnly);
      const action = form && new URL(form.action);
      return {url: location.href, timeOrigin: performance.timeOrigin, state: document.readyState,
        ready: Boolean(form?.method === "post" && !form.target && action?.origin === location.origin && action.pathname === expectedPath &&
          name?.form === form && editable(name) && editable(password) && visible(toggle) && !toggle?.matches(":disabled") &&
          visible(submit) && !submit?.matches(":disabled") && !["formaction", "formmethod", "formtarget"].some(attribute => submit?.hasAttribute(attribute)))};
    }, path.split("?")[0]);
    if (!originalDocument.ready || originalDocument.state !== "complete" || originalDocument.timeOrigin === previousDocument ||
        originalDocument.url !== observed.url()) throw error;
    page.off("response", observe);
    await info.attach("firefox-auth-form-navigation-recovery", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      path, status: observed.status(), responseAfter, elapsed: Date.now() - started, originalDocument,
      browser: page.context().browser()?.version(), action: "one superseding GET after verified original document"}), contentType: "application/json"});
    return await navigate();
  } finally {
    page.off("response", observe);
  }
}
