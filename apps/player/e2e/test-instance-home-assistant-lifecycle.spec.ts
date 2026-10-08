import {recordHomeAssistantEvidence} from "./home-assistant-document-evidence";
import {expect, test, type Page, type BrowserContext} from "@playwright/test";
import {configureTestInstance, createViewer, removeViewer} from "./test-instance-helpers";
import {openDocuments, closeDocuments, setting, observeAcceptedDocumentStates, nextDocumentClaim} from "./home-assistant-document-helpers";
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
    const renewed = nextDocumentClaim(first);
    void renewed.catch(() => {});
    await first.reload();
    const replacement = await renewed;
    expect(typeof replacement.id).toBe("string"); expect(replacement.id).not.toBe(before);
    docs.claims.add(replacement.claim);
    await expect.poll(() => accepted.some(state => state.id === replacement.id && state.claim === replacement.claim)).toBe(true);
    await expect.poll(async () => (await docs.live()).some(target => target.itemId === docs.ids[0] && target.id === replacement.id)).toBe(true);
    expect(await first.evaluate(claims => !Object.keys(localStorage).some(key => claims.some(claim => (localStorage.getItem(key) || "").includes(claim))), [...docs.claims])).toBe(true);
    await recordHomeAssistantEvidence(info, "actual-storage-degradation", {secureContext: true, distinctTargets: 2,
      deniedStorage: true, uuidAndLocksUnavailable: true, reloadChangesTarget: true, claimPersisted: false});
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
    const renewed = nextDocumentClaim(second);
    void occupied.catch(() => {}); void renewed.catch(() => {});
    const accepted = observeAcceptedDocumentStates(second);
    await second.goto(`/watch/${docs.ids[1]}`);
    expect((await occupied).status()).toBe(409);
    const replacement = await renewed;
    expect(typeof replacement.id).toBe("string"); expect(replacement.id).not.toBe(original);
    await expect.poll(() => accepted.some(state => state.id === replacement.id && state.claim === replacement.claim)).toBe(true);
    await expect.poll(async () => (await docs.live()).some(target => target.itemId === docs.ids[1] && target.id === replacement.id)).toBe(true);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    const live = await docs.live();
    expect(live.find(target => target.itemId === docs.ids[0])!.id).toBe(original);
    expect(live.find(target => target.itemId === docs.ids[1])!.id).toBe(replacement.id);
    await expect(second.locator("[data-home-assistant-lifetime]")).toHaveText(/fresh page received its own target/);
    await recordHomeAssistantEvidence(info, "actual-cloned-candidate", {occupiedStatus: 409, forked: true,
      originalTargetPreserved: true, distinctTargets: 2});
  } finally {await closeDocuments(page, [first, second]);}
});

test("real lost-release reload waits for lease expiry and renews only its original target", {tag: ["@smoke", "@routed-fault"]}, async ({page, browserName}, info) => {
  test.setTimeout(75_000);
  let first: Page | undefined, second: Page | undefined;
  try {
    const docs = await openDocuments(page); ({first, second} = docs);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    const original = (await docs.live()).find(target => target.itemId === docs.ids[0])!.id;
    let dropped = 0, conflicts = 0;
    const releasePath = `/api/v1/home-assistant/players/${original}/release`;
    // Block the real unload keepalive in Chromium's network stack. Page routing may
    // disappear with the departing document before that request is dispatched.
    const network = browserName === "chromium" ? await first.context().newCDPSession(first) : undefined;
    if (network) {
      await network.send("Network.enable");
      await network.send("Network.setBlockedURLs", {urls: [`*${releasePath}`]});
      first.on("requestfailed", request => {
        if (new URL(request.url()).pathname === releasePath &&
            request.failure()?.errorText === "net::ERR_BLOCKED_BY_CLIENT") dropped++;
      });
    } else {
      await first.context().route(`**${releasePath}`, route => {dropped++; return route.abort();});
    }
    first.on("response", response => {if (new URL(response.url()).pathname.endsWith("/players/claims") && response.status() === 409) conflicts++;});
    const renewed = nextDocumentClaim(first);
    void renewed.catch(() => {});
    const started = Date.now();
    await first.reload();
    const claim = await renewed;
    const elapsed = Date.now() - started;
    expect(claim.id).toBe(original);
    expect(claim.expiresIn).toBe(30);
    expect(dropped).toBeGreaterThan(0); expect(conflicts).toBeGreaterThan(0);
    expect(elapsed).toBeLessThanOrEqual(35_000);
    await expect.poll(async () => (await docs.live()).find(target => target.itemId === docs.ids[0])?.id).toBe(original);
    if (network) {
      await network.send("Network.setBlockedURLs", {urls: []});
      await network.detach();
    } else await first.context().unroute(`**${releasePath}`);

    await recordHomeAssistantEvidence(info, "actual-lost-release-expiry", {releaseDropped: true, realOccupiedReplies: conflicts,
      faultMechanism: network ? "Chromium network block" : "context route abort", sameCandidateRenewed: true, expiresIn: 30, elapsedMs: elapsed});
  } finally {await closeDocuments(page, [first, second]);}
});

test("actual authenticated Profile switch retires old document command effects", {tag: ["@smoke", "@routed-fault"]}, async ({page, browser}, info) => {
  test.setTimeout(75_000);
  let first: Page | undefined, second: Page | undefined, owner: Page | undefined, profile = "";
  let ownerContext: BrowserContext | undefined, viewerSwitched = false;
  let releaseCommand = () => {};
  const name = `R18 isolated Viewer ${info.workerIndex}`, password = "r18-fictional-viewer-password";
  try {
    const docs = await openDocuments(page); ({first, second} = docs);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    ownerContext = await browser.newContext({baseURL: new URL(page.url()).origin, storageState: await page.context().storageState()});
    owner = await ownerContext.newPage();
    profile = await test.step("create the authenticated Viewer fixture", () => createViewer(owner!, name, password));
    // Prepare the real login form before holding a reply, keeping the switch
    // within the product's unchanged five-second request budget.
    await test.step("prepare the ordinary Viewer login form", async () => {
      await page.goto("/login");
      await page.getByLabel("Name").fill(name);
      await page.getByLabel("Password", {exact: true}).fill(password);
    });
    const target = (await docs.live()).find(target => target.itemId === docs.ids[0])!.id;
    const before = await first.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime);
    const duration = await first.locator("video").evaluate((media: HTMLVideoElement) => media.duration);
    expect(Number.isFinite(duration) && duration > 2).toBe(true);
    const position = before < duration / 2 ? duration * .75 : duration * .25;
    expect(Math.abs(position - before)).toBeGreaterThan(.5);
    let release!: () => void, captured = false;
    const barrier = new Promise<void>(resolve => release = resolve);
    releaseCommand = release;
    await first.route(`**/api/v1/home-assistant/players/${target}`, async route => {
      const response = await route.fetch();
      if (response.status() === 200 && (await response.json()).command === "seek") {captured = true; await barrier;}
      try {await route.fulfill({response});} catch (error) {if (!first!.isClosed()) throw error;}
    });
    // Hold an actual accepted command response; do not fabricate a handler or payload.
    expect(await owner.evaluate(async ({id, position}) => {
      const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content || "";
      return (await fetch(`/api/v1/home-assistant/players/${id}/commands`, {method: "POST",
        headers: {"Content-Type": "application/json", "X-Kinosail-CSRF": csrf}, body: JSON.stringify({command: "seek", position})})).status;
    }, {id: target, position})).toBe(202);
    await test.step("capture a real accepted seek reply", () => expect.poll(() => captured, {timeout: 10_000}).toBe(true));
    const changed = first.waitForResponse(async response => new URL(response.url()).pathname === "/api/v1/me" &&
      response.status() === 200 && (await response.json()).viewer.id === profile);
    const delivered = first.waitForResponse(async response => new URL(response.url()).pathname === `/api/v1/home-assistant/players/${target}` &&
      response.status() === 200 && (await response.json()).command === "seek");
    void delivered.catch(() => {});
    void changed.catch(() => {});
    try {
      await test.step("switch Profile through the prepared login form", async () => {
        await page.getByRole("button", {name: "Sign in", exact: true}).click();
        await page.waitForURL(url => url.pathname !== "/login");
        const viewer = await page.request.get("/api/v1/me");
        expect(viewer.status()).toBe(200);
        expect((await viewer.json()).viewer.id).toBe(profile);
        viewerSwitched = true;
      });
    } finally {release();}
    expect((await delivered).status()).toBe(200);
    await changed;
    await expect(first.locator("[data-home-assistant-status]")).toHaveAttribute("data-home-assistant-status", "stopped");
    let lateStates = 0;
    first.on("request", request => {if (request.method() === "PUT" && new URL(request.url()).pathname === `/api/v1/home-assistant/players/${target}`) lateStates++;});
    await first.waitForTimeout(11_000);
    expect(lateStates).toBe(0);
    expect(await first.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime)).toBeCloseTo(before, 2);
    await recordHomeAssistantEvidence(info, "actual-authenticated-profile-retirement", {publicViewerChanged: true,
      commandAcceptedBeforeSwitch: 202, commandDeliveredAfterSwitch: 200, actualCommandReplyHeld: true,
      seekSeparatedByMoreThanHalfSecond: true, lateStateWrites: 0, mediaUnchanged: true});
  } finally {
    releaseCommand();
    try {
      for (const document of [first, second]) if (document && !document.isClosed()) await document.close();
    } finally {
      try {
        const management = owner || (!viewerSwitched ? page : undefined);
        if (management) {try {await setting(management, false);} finally {if (profile) await removeViewer(management, profile);}}
      } finally {await ownerContext?.close();}
    }
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
    await recordHomeAssistantEvidence(info, "actual-history-admission", {persisted, sourceRestored,
      ordinaryHistoryTargetRetained: !persisted, playableBFCacheQualified: false, ordinaryHTTPQualified: false});
  } finally {await closeDocuments(page, [first, second]);}
});
