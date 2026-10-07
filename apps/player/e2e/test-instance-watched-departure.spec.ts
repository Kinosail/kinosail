import {expect, test} from "@playwright/test";
import {configureTestInstance, firstPlayable, login} from "./test-instance-helpers";

configureTestInstance();
// The explicit form must close its page even before playback JavaScript loads.
// This covers the rendered form/CSRF contract, independently of media timing.
test.use({javaScriptEnabled: false, serviceWorkers: "block"});

test("usable login form authenticates while an unrelated image remains pending", {tag: "@smoke"}, async ({page}) => {
  test.setTimeout(15000);
  let release!: () => void;
  const pending = new Promise<void>(resolve => release = resolve);
  await page.route("**/diagnostic-held-login-image", async route => {
    await pending;
    await route.fulfill({status: 204});
  });
  await page.route("**/login", async route => {
    if (route.request().method() !== "GET") return route.continue();
    const response = await route.fetch();
    expect(response.status()).toBe(200);
    const body = await response.text();
    await route.fulfill({response, body: body.replace("</body>", '<img src="/diagnostic-held-login-image" alt=""></body>')});
  });
  try {
    await login(page);
    const authenticated = await page.request.get("/api/v1/me");
    expect(authenticated.status()).toBe(200);
  } finally {release();}
});

test("Mark watched without JavaScript rejects the current page's first late progress write", {tag: "@smoke"}, async ({page}, testInfo) => {
  await login(page);
  const watch = await firstPlayable(page);
  const id = watch.split("/").at(-1)!;
  const origin = new URL(page.url()).origin;
  const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute("content");
  expect(csrf).toBeTruthy();
  const headers = {Origin: origin, "X-Kinosail-CSRF": csrf!};
  const seed = await page.request.put(`/api/v1/items/${id}/progress`, {
    headers, data: {seconds: 12, watched: false, session: `prior-page-${testInfo.project.name}-${testInfo.repeatEachIndex}-${testInfo.retry}`, revision: 1},
  });
  expect(seed.status()).toBe(200);
  const departingWrites: string[] = [];
  page.on("request", request => {
    if (request.method() === "POST" && new URL(request.url()).pathname === `/progress/${id}`) departingWrites.push("progress");
  });
  await page.goto(watch, {waitUntil: "domcontentloaded"});
  const session = await page.locator("video").getAttribute("data-playback-session");
  expect(session).toBeTruthy();
  await expect(page.locator(`form[action="/watched/${id}"] input[name="session"]`)).toHaveValue(session!);
  await page.getByRole("button", {name: "Mark watched", exact: true}).click();
  await expect(page.getByRole("button", {name: "Mark unwatched", exact: true})).toBeVisible();
  expect(departingWrites).toEqual([]);
  const beforeResponse = await page.request.get(`/api/v1/items/${id}`);
  expect(beforeResponse.status()).toBe(200);
  const before = (await beforeResponse.json()).item.progress;
  expect(before.watched).toBe(true);
  expect(before.seconds || 0).toBe(0);
  // Actual HTTP, with the old page identity and no prior write from that page.
  // This represents the observed pagehide payload; it is not a media-event proof.
  const late = await page.request.put(`/api/v1/items/${id}/progress`, {
    headers, data: {seconds: 0.115, watched: false, session, revision: 1},
  });
  expect(late.status()).toBe(409);
  const afterResponse = await page.request.get(`/api/v1/items/${id}`);
  expect(afterResponse.status()).toBe(200);
  expect((await afterResponse.json()).item.progress).toEqual(before);
  expect(departingWrites).toEqual([]);
  await testInfo.attach("watched-form-first-write", {body: JSON.stringify({
    javascript: false, priorCurrentPageWrites: 0, lateHTTPStatus: late.status(), stateUnchanged: true,
  }), contentType: "application/json"});
});
