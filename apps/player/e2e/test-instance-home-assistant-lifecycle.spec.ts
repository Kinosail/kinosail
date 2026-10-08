import {expect, test, type Page, type BrowserContext} from "@playwright/test";
import {configureTestInstance, createViewer, loginViewer, newViewerPage, removeViewer} from "./test-instance-helpers";
import {openDocuments, closeDocuments, setting, observeAcceptedDocumentStates, observeActualDocumentClaims} from "./home-assistant-document-helpers";
import {inspectDocumentStatus} from "./home-assistant-document-inspection";

configureTestInstance();

test("real document targets work with denied storage and unavailable UUID and locks", {tag: "@smoke"}, async ({page}, info) => {
  let first: Page | undefined, second: Page | undefined;
  try {
    const docs = await openDocuments(page, async document => document.addInitScript(() => {
      Object.defineProperty(window, "sessionStorage", {get() {throw new DOMException("Unavailable", "SecurityError");}});
      Object.defineProperty(Crypto.prototype, "randomUUID", {value: undefined});
      Object.defineProperty(navigator, "locks", {value: undefined});
    }));
    ({first, second} = docs);
    expect(await first.evaluate(() => ({secure: isSecureContext, uuid: typeof crypto.randomUUID, locks: typeof navigator.locks})))
      .toEqual({secure: true, uuid: "undefined", locks: "undefined"});
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    await expect(first.locator("[data-home-assistant-lifetime]")).toHaveText(
      "Browser storage is unavailable. Reloading creates a new Home Assistant target.");
    await inspectDocumentStatus(first, info, "connected");
    const before = (await docs.live()).find(target => target.itemId === docs.ids[0])!.id;
    const accepted = observeAcceptedDocumentStates(first);
    const actualClaims = await observeActualDocumentClaims(first);
    const renewed = first.waitForResponse(response => new URL(response.url()).pathname.endsWith("/players/claims") && response.status() === 201);
    void renewed.catch(() => {});
    await first.reload();
    expect((await renewed).status()).toBe(201);
    const replacement = actualClaims.at(-1)!;
    expect(typeof replacement.id).toBe("string"); expect(replacement.id).not.toBe(before);
    docs.claims.add(replacement.claim);
    await expect.poll(() => accepted.some(state => state.id === replacement.id && state.claim === replacement.claim)).toBe(true);
    await expect.poll(async () => (await docs.live()).some(target => target.itemId === docs.ids[0] && target.id === replacement.id)).toBe(true);
    expect(await first.evaluate(claims => !Object.keys(localStorage).some(key => claims.some(claim => (localStorage.getItem(key) || "").includes(claim))), [...docs.claims])).toBe(true);
    await info.attach("actual-storage-degradation", {body: JSON.stringify({secureContext: true, distinctTargets: 2,
      deniedStorage: true, uuidAndLocksUnavailable: true, reloadChangesTarget: true, claimPersisted: false}), contentType: "application/json"});
  } finally {await closeDocuments(page, [first, second]);}
});

test("real cloned document candidate forks after occupied claim without stealing the live target", {tag: "@smoke"}, async ({page}, info) => {
  let first: Page | undefined, second: Page | undefined;
  try {
    const docs = await openDocuments(page); ({first, second} = docs);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    const original = (await docs.live()).find(target => target.itemId === docs.ids[0])!.id;
    await second.goto("/?view=movies");
    // Seed only the reload candidate; the cloned page must acquire fresh authority.
    const profile = await first.evaluate(() => document.body.dataset.viewerProfile || "");
    await second.addInitScript(({profile, candidate}) => sessionStorage.setItem(`kinosail-home-assistant-document:${profile}`, candidate),
      {profile, candidate: original});
    const occupied = second.waitForResponse(response => new URL(response.url()).pathname.endsWith("/home-assistant/players/claims") && response.status() === 409);
    const renewed = second.waitForResponse(response => new URL(response.url()).pathname.endsWith("/players/claims") && response.status() === 201);
    void occupied.catch(() => {}); void renewed.catch(() => {});
    const accepted = observeAcceptedDocumentStates(second);
    const actualClaims = await observeActualDocumentClaims(second);
    await second.goto(`/watch/${docs.ids[1]}`);
    expect((await occupied).status()).toBe(409);
    expect((await renewed).status()).toBe(201);
    const replacement = actualClaims.at(-1)!;
    expect(typeof replacement.id).toBe("string"); expect(replacement.id).not.toBe(original);
    await expect.poll(() => accepted.some(state => state.id === replacement.id && state.claim === replacement.claim)).toBe(true);
    await expect.poll(async () => (await docs.live()).some(target => target.itemId === docs.ids[1] && target.id === replacement.id)).toBe(true);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    const live = await docs.live();
    expect(live.find(target => target.itemId === docs.ids[0])!.id).toBe(original);
    expect(live.find(target => target.itemId === docs.ids[1])!.id).toBe(replacement.id);
    await expect(second.locator("[data-home-assistant-lifetime]")).toHaveText(/fresh page received its own target/);
    await info.attach("actual-cloned-candidate", {body: JSON.stringify({occupiedStatus: 409, forked: true,
      originalTargetPreserved: true, distinctTargets: 2}), contentType: "application/json"});
  } finally {await closeDocuments(page, [first, second]);}
});

test("real lost-release reload waits for lease expiry and renews only its original target", {tag: ["@smoke", "@routed-fault"]}, async ({page}, info) => {
  test.setTimeout(75_000);
  let first: Page | undefined, second: Page | undefined, releasePath = "";
  try {
    const docs = await openDocuments(page); ({first, second} = docs);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    const original = (await docs.live()).find(target => target.itemId === docs.ids[0])!.id;
    const accepted = observeAcceptedDocumentStates(first);
    const actualClaims = await observeActualDocumentClaims(first);
    let dropped = 0, conflicts = 0;
    releasePath = `**/api/v1/home-assistant/players/${original}/release`;
    await page.context().route(releasePath, route => {
      if (route.request().method() !== "POST") return route.continue();
      dropped++; return route.abort();
    });
    first.on("response", response => {if (new URL(response.url()).pathname.endsWith("/players/claims") && response.status() === 409) conflicts++;});
    const renewed = first.waitForResponse(response => new URL(response.url()).pathname.endsWith("/players/claims") && response.status() === 201, {timeout: 40_000});
    void renewed.catch(() => {});
    const started = Date.now();
    await first.reload();
    expect((await renewed).status()).toBe(201);
    const claim = actualClaims.at(-1)!;
    const elapsed = Date.now() - started;
    expect(claim.id).toBe(original);
    expect(claim.expiresIn).toBe(30);
    expect(dropped).toBeGreaterThan(0); expect(conflicts).toBeGreaterThan(0);
    expect(elapsed).toBeLessThanOrEqual(35_000);
    await expect.poll(() => accepted.some(state => state.id === claim.id && state.claim === claim.claim)).toBe(true);
    await expect.poll(async () => (await docs.live()).find(target => target.itemId === docs.ids[0])?.id).toBe(original);
    await page.context().unroute(releasePath); releasePath = "";
    await info.attach("actual-lost-release-expiry", {body: JSON.stringify({releaseDropped: true, realOccupiedReplies: conflicts,
      sameCandidateRenewed: true, expiresIn: 30, elapsedMs: elapsed}), contentType: "application/json"});
  } finally {
    if (releasePath) await page.context().unroute(releasePath);
    await closeDocuments(page, [first, second]);
  }
});

test("actual authenticated Profile switch retires old document command effects", {tag: ["@smoke", "@routed-fault"]}, async ({page, browser}, info) => {
  test.setTimeout(75_000);
  let first: Page | undefined, second: Page | undefined, owner: Page | undefined, viewer: Page | undefined, profile = "";
  let ownerContext: BrowserContext | undefined, primary: unknown;
  let releaseCommand = () => {};
  const name = `R18 isolated Viewer ${info.workerIndex}`, password = "r18-fictional-viewer-password";
  try {
    const docs = await openDocuments(page); ({first, second} = docs);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    const baseURL = new URL(page.url()).origin;
    ownerContext = await browser.newContext({baseURL, storageState: await page.context().storageState()});
    owner = await ownerContext.newPage();
    profile = await createViewer(owner, name, password);
    // Authenticate through the real UI before starting the bounded held reply.
    viewer = await newViewerPage(browser, baseURL);
    await loginViewer(viewer, name, password);
    const authenticated = await viewer.request.get("/api/v1/me");
    expect(authenticated.status()).toBe(200);
    expect((await authenticated.json()).viewer.id === profile, "real UI authenticated the fictional Viewer").toBe(true);
    const sessions = (await viewer.context().cookies(baseURL)).filter(cookie => cookie.name === "__Host-kinosail_player_session");
    expect(sessions.length, "the server issued exactly one app-owned session cookie").toBe(1);
    const target = (await docs.live()).find(target => target.itemId === docs.ids[0])!.id;
    const before = await first.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime);
    const duration = await first.locator("video").evaluate((media: HTMLVideoElement) => media.duration);
    expect(Number.isFinite(duration) && duration > 2).toBe(true);
    const position = before < duration / 2 ? duration * .75 : duration * .25;
    expect(Math.abs(position - before)).toBeGreaterThan(.5);
    let release!: () => void, capturedAt = 0;
    const barrier = new Promise<void>(resolve => release = resolve);
    releaseCommand = release;
    await first.route(`**/api/v1/home-assistant/players/${target}`, async route => {
      if (route.request().method() !== "PUT") return route.continue();
      const response = await route.fetch();
      if (response.status() === 200 && (await response.json()).command === "seek") {capturedAt = Date.now(); await barrier;}
      try {await route.fulfill({response});} catch (error) {if (!first!.isClosed()) throw error;}
    });
    expect(await owner.evaluate(async ({id, position}) => {
      const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content || "";
      return (await fetch(`/api/v1/home-assistant/players/${id}/commands`, {method: "POST",
        headers: {"Content-Type": "application/json", "X-Kinosail-CSRF": csrf}, body: JSON.stringify({command: "seek", position})})).status;
    }, {id: target, position})).toBe(202);
    await expect.poll(() => capturedAt, {timeout: 15_000, message: "actual accepted seek reply reached the held route"}).toBeGreaterThan(0);
    const changed = first.waitForResponse(response => new URL(response.url()).pathname === "/api/v1/me" &&
      response.status() === 200, {timeout: 5_000});
    const delivered = first.waitForResponse(response => new URL(response.url()).pathname === `/api/v1/home-assistant/players/${target}` &&
      response.request().method() === "PUT" && response.status() === 200, {timeout: 5_000});
    void delivered.catch(() => {}); void changed.catch(() => {});
    let lateStates = 0;
    first.on("request", request => {if (request.method() === "PUT" && new URL(request.url()).pathname === `/api/v1/home-assistant/players/${target}`) lateStates++;});
    // Overwrite only genuine server authority; never introduce an unauthenticated gap.
    await page.context().addCookies(sessions);
    const switched = await first.request.get("/api/v1/me", {timeout: 2_000});
    expect(switched.status()).toBe(200);
    expect((await switched.json()).viewer.id === profile, "shared session identifies the authenticated Viewer").toBe(true);
    const heldMs = Date.now() - capturedAt;
    expect(heldMs, "deliver inside the unchanged five-second request deadline").toBeLessThan(4_000);
    release();
    expect((await delivered).status()).toBe(200);
    expect((await (await changed).json()).viewer.id === profile, "the actual command adapter observed the new Viewer").toBe(true);
    await expect(first.locator("[data-home-assistant-status]")).toHaveAttribute("data-home-assistant-status", "stopped");
    await first.waitForTimeout(11_000);
    expect(lateStates).toBe(0);
    expect(await first.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime)).toBeCloseTo(before, 2);
    await info.attach("actual-authenticated-profile-retirement", {body: JSON.stringify({realUIAuthenticatedViewer: true,
      serverIssuedSessionReplaced: true, publicViewerChanged: true, commandAcceptedBeforeSwitch: 202,
      commandDeliveredAfterSwitch: 200, actualCommandReplyHeld: true, heldMs,
      seekSeparatedByMoreThanHalfSecond: true, lateStateWrites: 0, mediaUnchanged: true}), contentType: "application/json"});
  } catch (error) {primary = error; throw error;} finally {
    releaseCommand();
    let cleanup: unknown;
    const attempt = async (action: () => Promise<unknown>) => {try {await action();} catch (error) {cleanup ||= error;}};
    for (const document of [first, second]) if (document && !document.isClosed()) await attempt(() => document.close());
    if (owner) {
      await attempt(() => setting(owner!, false));
      if (profile) await attempt(() => removeViewer(owner!, profile));
    }
    if (viewer) await attempt(() => viewer!.context().close());
    if (ownerContext) await attempt(() => ownerContext!.close());
    if (!primary && cleanup) throw cleanup;
  }
});

test("rendered document claim failure exposes accessible Retry and recovers with a real claim", {tag: ["@smoke", "@routed-fault"]}, async ({page}, info) => {
  let first: Page | undefined, second: Page | undefined, failing = true;
  try {
    const docs = await openDocuments(page, document => document.route("**/home-assistant/players/claims", route => failing
      ? route.fulfill({status: 503}) : route.continue()));
    ({first, second} = docs);
    await expect(first.locator("[data-home-assistant-status]")).toHaveAttribute("data-home-assistant-status", "unavailable");
    await inspectDocumentStatus(first, info, "unavailable");
    failing = false;
    const claim = first.waitForResponse(response => new URL(response.url()).pathname.endsWith("/players/claims") && response.status() === 201);
    void claim.catch(() => {});
    await first.getByRole("button", {name: "Retry Home Assistant", exact: true}).press("Enter");
    expect((await claim).status()).toBe(201);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    await inspectDocumentStatus(first, info, "connected");
  } finally {await closeDocuments(page, [first, second]);}
});

test("actual history return records document reentry separately from persisted playable BFCache admission", {tag: "@smoke"}, async ({page}, info) => {
  let first: Page | undefined, second: Page | undefined;
  try {
    const docs = await openDocuments(page, document => document.addInitScript(() => {
      (window as any).r18PageShowPersisted = false;
      addEventListener("pageshow", event => {(window as any).r18PageShowPersisted = event.persisted;});
    }));
    ({first, second} = docs);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    const original = (await docs.live()).find(target => target.itemId === docs.ids[0])!.id;
    await first.goto("/?view=movies");
    await first.goBack();
    const persisted = await first.evaluate(() => (window as any).r18PageShowPersisted === true);
    if (!persisted) {
      await expect.poll(async () => (await docs.live()).find(target => target.itemId === docs.ids[0])?.id, {timeout: 40_000}).toBe(original);
      await expect.poll(() => first!.locator("video").evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThanOrEqual(2);
    }
    const sourceRestored = await first.locator("video").evaluate((media: HTMLVideoElement) => !!media.getAttribute("src") && media.readyState >= 2);
    info.annotations.push({type: "admission", description: persisted && sourceRestored
      ? "persisted return observed; full playable lifecycle qualification remains separate"
      : "persisted playable BFCache unadmitted; media restoration belongs to presentation/queue owner"});
    await info.attach("actual-history-admission", {body: JSON.stringify({persisted, sourceRestored,
      ordinaryHistoryTargetRetained: !persisted, playableBFCacheQualified: false, ordinaryHTTPQualified: false}), contentType: "application/json"});
  } finally {await closeDocuments(page, [first, second]);}
});
