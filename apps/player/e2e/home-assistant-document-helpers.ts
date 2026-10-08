import {expect, type Page} from "@playwright/test";
import {login} from "./test-instance-helpers";

type DocumentClaim = {id: string; claim: string; expiresIn: number};
const actualDocumentClaims = new WeakMap<Page, DocumentClaim[]>();

// Capture genuine claim bodies before the browser can retire a response resource.
async function prepareDocumentClaims(page: Page, privateClaims: Set<string>) {
  const captured: DocumentClaim[] = [];
  actualDocumentClaims.set(page, captured);
  await page.route("**/api/v1/home-assistant/players/claims", async route => {
    if (route.request().method() !== "POST") return route.continue();
    const response = await route.fetch();
    if (response.status() === 201) {
      const body = await response.json();
      const valid = typeof body?.id === "string" && /^[A-Za-z0-9_-]{1,64}$/.test(body.id) &&
        typeof body?.claim === "string" && /^[A-Za-z0-9_-]{20,64}$/.test(body.claim) && body.expiresIn === 30;
      expect(valid, "actual server claim has the admitted public shape").toBe(true);
      expect(captured.length < 100, "private claim observations stay bounded").toBe(true);
      captured.push({id: body.id, claim: body.claim, expiresIn: body.expiresIn});
      privateClaims.add(body.claim);
    }
    await route.fulfill({response});
  });
}

export function nextDocumentClaim(page: Page, timeout = 40_000): Promise<DocumentClaim> {
  const captured = actualDocumentClaims.get(page);
  if (!captured) throw new Error("actual claim observation was not prepared");
  const index = captured.length;
  const body = page.waitForResponse(response => new URL(response.url()).pathname ===
    "/api/v1/home-assistant/players/claims" && response.request().method() === "POST" && response.status() === 201,
    {timeout}).then(() => {
      expect(captured.length > index, "actual server body precedes browser delivery").toBe(true);
      return captured[index];
    });
  void body.catch(() => {});
  return body;
}

type Target = {id: string; itemId: string; position: number; duration: number};

// Test-private observations. Claim values never enter attachments or diagnostics.
export function observeAcceptedDocumentStates(page: Page) {
  const accepted: Array<{id: string; claim: string}> = [];
  page.on("response", async response => {
    const match = new URL(response.url()).pathname.match(/^\/api\/v1\/home-assistant\/players\/([A-Za-z0-9_-]+)$/);
    if (!match || response.request().method() !== "PUT" || response.status() !== 200) return;
    try {
      const claim = (await response.request().allHeaders())["x-kinosail-player-claim"] || "";
      if (accepted.length < 100) accepted.push({id: match[1], claim});
    } catch (_) { /* A retired page cannot produce an accepted observation. */ }
  });
  return accepted;
}

export async function setting(page: Page, enabled: boolean) {
  expect(await page.evaluate(async enabled => {
    const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content || "";
    return (await fetch("/api/v1/settings/home-assistant", {method: "PUT",
      headers: {"Content-Type": "application/json", "X-Kinosail-CSRF": csrf},
      body: JSON.stringify({enabled})})).status;
  }, enabled)).toBe(200);
}

export async function targets(page: Page): Promise<Target[]> {
  const response = await page.request.get("/api/v1/home-assistant/players");
  expect(response.status()).toBe(200);
  return (await response.json()).players;
}

export async function openDocuments(page: Page, prepare?: (document: Page) => Promise<void>, options: {initialClaimsRequired?: boolean} = {}) {
  await login(page);
  await setting(page, true);
  const existing = new Set((await targets(page)).map(target => target.id));
  const library = await page.request.get("/api/v1/library?view=movies");
  expect(library.status()).toBe(200);
  const items = (await library.json()).items as Array<{id: string; title: string}>;
  const ids = ["R18 Fictional Alpha", "R18 Fictional Beta"].map(title => {
    const item = items.find(item => item.title === title);
    expect(item, "the exact admitted R18 fixture must be indexed").toBeTruthy();
    return item!.id;
  });
  const first = await page.context().newPage();
  let second: Page | undefined;
  try {
  second = await page.context().newPage();
  for (const document of [first, second]) await prepare?.(document);
  const claims = new Set<string>();
  if (options.initialClaimsRequired !== false) {
    for (const document of [first, second]) await prepareDocumentClaims(document, claims);
  }
  const live = async () => (await targets(page)).filter(target => ids.includes(target.itemId) && !existing.has(target.id));
  const initialClaims = options.initialClaimsRequired === false ? [] : [nextDocumentClaim(first), nextDocumentClaim(second)];
  await first.goto(`/watch/${ids[0]}`);
  await second.goto(`/watch/${ids[1]}`);
  for (const claim of await Promise.all(initialClaims)) {
    expect(typeof claim.claim === "string" && claim.claim.length >= 20).toBe(true);
    expect(claim.expiresIn).toBe(30);
    claims.add(claim.claim);
  }
  for (const document of [first, second]) {
    await expect(document.locator("video")).toHaveAttribute("data-home-assistant", "true");
    await expect.poll(() => document.locator("video").evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThanOrEqual(2);
    await document.locator("video").evaluate((media: HTMLVideoElement) => media.pause());
  }
  return {first, second, ids, live, claims};
  } catch (error) {
    await first.close();
    if (second) await second.close();
    throw error;
  }
}

export async function closeDocuments(page: Page, documents: Array<Page | undefined>) {
  for (const document of documents) if (document && !document.isClosed()) await document.close();
  await setting(page, false);
}
