import {recordHomeAssistantEvidence, cleanupHomeAssistantFixture} from "./home-assistant-document-evidence";
import {expect, test, type Page, type BrowserContext, type CDPSession, type Request, type Response} from "@playwright/test";
import {configureTestInstance, createViewer, loginViewer, removeViewer} from "./test-instance-helpers";
import {openDocuments, closeDocuments, setting, observeAcceptedDocumentStates, nextDocumentClaim, observeNativeDocumentRetirement, observeNativeReleaseBoundary} from "./home-assistant-document-helpers";
import {inspectDocumentStatus} from "./home-assistant-document-inspection";

import {installNativeReleaseDiagnostic} from "../../../packages/webassets/home-assistant-release-diagnostic-fixture.mjs";

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
  let first: Page | undefined, second: Page | undefined, network: CDPSession | undefined, releaseRoute = "", primary: unknown;
  let releaseBoundary: Awaited<ReturnType<typeof observeNativeReleaseBoundary>> | undefined;
  let failedRelease: ((request: Request) => void) | undefined, occupiedReply: ((response: Response) => void) | undefined;
  try {
    const docs = await openDocuments(page, document => document.addInitScript(installNativeReleaseDiagnostic)); ({first, second} = docs);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    const original = (await docs.live()).find(target => target.itemId === docs.ids[0])!.id;
    const accepted = observeAcceptedDocumentStates(first);
    let dropped = 0, contextAborts = 0, conflicts = 0;
    const releaseRequests = new Set<string>(), blockedRequests = new Set<string>();
    const releasePath = `/api/v1/home-assistant/players/${original}/release`;
    // The context survives the departing document; count only an actual native POST
    // after its transport abort completes, never route installation or intent.
    releaseRoute = `**${releasePath}`;
    await page.context().route(releaseRoute, async route => {
      const request = route.request();
      if (request.method() !== "POST" || new URL(request.url()).pathname !== releasePath) return route.continue();
      expect(docs.claims.has(request.headers()["x-kinosail-player-claim"] || ""), "native release uses genuine document authority").toBe(true);
      expect(contextAborts < 8, "context release observations stay bounded").toBe(true);
      await route.abort("blockedbyclient");
      contextAborts++;
    });
    network = browserName === "chromium" ? await first.context().newCDPSession(first) : undefined;
    if (network) {
      network.on("Network.requestWillBeSent", ({request, requestId}) => {
        if (request.method === "POST" && new URL(request.url).pathname === releasePath) {
          expect(releaseRequests.size < 8, "release observations stay bounded").toBe(true);
          releaseRequests.add(requestId);
        }
      });
      network.on("Network.loadingFailed", ({requestId, errorText, blockedReason}) => {
        if (releaseRequests.has(requestId) && errorText === "net::ERR_BLOCKED_BY_CLIENT" && blockedReason === "inspector") blockedRequests.add(requestId);
      });
      await network.send("Network.enable");
      await network.send("Network.setBlockedURLs", {urls: [`*${releasePath}`]});
    }
    failedRelease = request => {
      if (request.method() === "POST" && new URL(request.url()).pathname === releasePath &&
          request.failure()?.errorText === "net::ERR_BLOCKED_BY_CLIENT") dropped++;
    };
    occupiedReply = response => {if (new URL(response.url()).pathname.endsWith("/players/claims") && response.status() === 409) conflicts++;};
    first.on("requestfailed", failedRelease); first.on("response", occupiedReply);
    const renewed = nextDocumentClaim(first);
    void renewed.catch(() => {});
    const retirement = network ? await observeNativeDocumentRetirement(network) : undefined;
    releaseBoundary = network ? await observeNativeReleaseBoundary(network, releasePath, docs.claims) : undefined;
    await first.evaluate(path => (window as any).__kinosailReleaseDiagnosticTarget(path), releasePath);
    retirement?.begin();
    const started = Date.now();
    await first.reload();
    const claim = await renewed;
    const elapsed = Date.now() - started;
    expect(claim.id).toBe(original);
    expect(claim.expiresIn).toBe(30);
    dropped = Math.max(dropped, blockedRequests.size, contextAborts);
    await recordHomeAssistantEvidence(info, "actual-lost-release-prerequisite", {releaseAttempts: Math.max(releaseRequests.size, contextAborts),
      observedBlockedFailures: dropped, contextAborts, protocolBlockedFailures: blockedRequests.size,
      actualOccupiedReplies: conflicts, sameCandidateRenewed: claim.id === original, expiresIn: claim.expiresIn, elapsedMs: elapsed,
      nativeEndpointDiagnostic: await first.evaluate(() => (window as any).__kinosailReleaseDiagnostic()),
      mainContextLifecycle: retirement?.snapshot() || {protocolUnavailable: true},
      nativeRequestBoundary: releaseBoundary?.snapshot() || {protocolUnavailable: true}});
    expect(dropped).toBeGreaterThan(0); expect(conflicts).toBeGreaterThan(0);
    expect(elapsed).toBeLessThanOrEqual(35_000);
    await expect.poll(() => accepted.some(state => state.id === claim.id && state.claim === claim.claim)).toBe(true);
    await expect.poll(async () => (await docs.live()).find(target => target.itemId === docs.ids[0])?.id).toBe(original);
    await recordHomeAssistantEvidence(info, "actual-lost-release-expiry", {releaseDropped: true, realOccupiedReplies: conflicts,
      faultMechanism: network ? "Browser context abort + Chromium network block" : "Browser context abort",
      contextAborts, protocolBlockedFailures: blockedRequests.size, sameCandidateRenewed: true, expiresIn: 30, elapsedMs: elapsed});
  } catch (error) {primary = error; throw error;} finally {
    await cleanupHomeAssistantFixture(info, primary, [
      {operation: "remove-release-observers", action: async () => {
        if (first && failedRelease) first.off("requestfailed", failedRelease);
        if (first && occupiedReply) first.off("response", occupiedReply);
      }},
      {operation: "remove-native-boundary-observer", action: async () => {await releaseBoundary?.close();}},
      {operation: "remove-release-route", action: async () => {if (releaseRoute) await page.context().unroute(releaseRoute);}},
      {operation: "remove-network-block", action: async () => {if (network) await network.send("Network.setBlockedURLs", {urls: []});}},
      {operation: "detach-network-observer", action: async () => {await network?.detach();}},
      {operation: "close-first-document", action: async () => {if (first && !first.isClosed()) await first.close();}},
      {operation: "close-second-document", action: async () => {if (second && !second.isClosed()) await second.close();}},
      {operation: "disable-home-assistant-fixture", action: () => setting(page, false)},
    ]);
  }
});

test("actual authenticated Profile switch retires old document command effects", {tag: ["@smoke", "@routed-fault"]}, async ({page, browser}, info) => {
  test.setTimeout(75_000);
  let first: Page | undefined, second: Page | undefined, owner: Page | undefined, viewer: Page | undefined, profile = "";
  let ownerContext: BrowserContext | undefined, viewerContext: BrowserContext | undefined, primary: unknown, viewerSwitched = false;
  let releaseCommand = () => {};
  const name = `R18 isolated Viewer ${info.workerIndex}`, password = "r18-fictional-viewer-password";
  try {
    const docs = await openDocuments(page); ({first, second} = docs);
    await expect.poll(async () => (await docs.live()).length).toBe(2);
    const baseURL = new URL(page.url()).origin;
    ownerContext = await browser.newContext({baseURL, storageState: await page.context().storageState()});
    owner = await ownerContext.newPage();
    profile = await test.step("create the authenticated Viewer fixture", () => createViewer(owner!, name, password));
    // Authenticate through the real UI before starting the bounded held reply.
    viewerContext = await browser.newContext({baseURL, ignoreHTTPSErrors: false});
    await viewerContext.addInitScript(() => Object.defineProperty(PublicKeyCredential, "isConditionalMediationAvailable", {value: async () => false}));
    viewer = await viewerContext.newPage();
    await test.step("authenticate the separate Viewer through the real login UI", () => loginViewer(viewer!, name, password));
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
    let release!: () => void, capturedAt = 0, requestStarted = 0;
    let heldRequest: Request | undefined, changedRequest: Request | undefined;
    const barrier = new Promise<void>(resolve => release = resolve);
    releaseCommand = release;
    // Observe the adapter's genuine server identity reply before the browser can
    // retire its response resource; deliver the unchanged response.
    await first.route("**/api/v1/me", async route => {
      if (route.request().method() !== "GET") return route.continue();
      const response = await route.fetch();
      if (response.status() === 200 && (await response.json()).viewer.id === profile) changedRequest = route.request();
      await route.fulfill({response});
    });
    await first.route(`**/api/v1/home-assistant/players/${target}`, async route => {
      if (route.request().method() !== "PUT") return route.continue();
      const started = Date.now();
      const response = await route.fetch();
      if (response.status() === 200 && (await response.json()).command === "seek") {
        heldRequest = route.request(); requestStarted = started; capturedAt = Date.now(); await barrier;
      }
      try {await route.fulfill({response});} catch (error) {if (!first!.isClosed()) throw error;}
    });
    expect(await owner.evaluate(async ({id, position}) => {
      const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content || "";
      return (await fetch(`/api/v1/home-assistant/players/${id}/commands`, {method: "POST",
        headers: {"Content-Type": "application/json", "X-Kinosail-CSRF": csrf}, body: JSON.stringify({command: "seek", position})})).status;
    }, {id: target, position})).toBe(202);
    await test.step("capture a real accepted seek reply", () => expect.poll(() => capturedAt, {timeout: 10_000}).toBeGreaterThan(0));
    const changed = first.waitForResponse(response => new URL(response.url()).pathname === "/api/v1/me" &&
      response.request() === changedRequest && response.status() === 200, {timeout: 5_000});
    const delivered = first.waitForResponse(response => new URL(response.url()).pathname === `/api/v1/home-assistant/players/${target}` &&
      response.request() === heldRequest && response.request().method() === "PUT" && response.status() === 200, {timeout: 5_000});
    void delivered.catch(() => {}); void changed.catch(() => {});
    let lateStates = 0;
    first.on("request", request => {if (request.method() === "PUT" && new URL(request.url()).pathname === `/api/v1/home-assistant/players/${target}`) lateStates++;});
    // Overwrite only genuine server authority; never introduce an unauthenticated gap.
    await page.context().addCookies(sessions); viewerSwitched = true;
    const switched = await first.request.get("/api/v1/me", {timeout: 2_000});
    expect(switched.status()).toBe(200);
    expect((await switched.json()).viewer.id === profile, "shared session identifies the authenticated Viewer").toBe(true);
    expect(Date.now() - requestStarted, "session verification leaves deadline margin").toBeLessThan(4_000);
    release();
    expect((await delivered).status()).toBe(200);
    const heldMs = Date.now() - requestStarted;
    expect(heldMs, "actual request including server fetch and browser delivery stays inside five seconds").toBeLessThan(4_000);
    expect((await changed).request() === changedRequest, "the actual command adapter received the exact new Viewer server reply").toBe(true);
    await expect(first.locator("[data-home-assistant-status]")).toHaveAttribute("data-home-assistant-status", "stopped");
    await first.waitForTimeout(11_000);
    expect(lateStates).toBe(0);
    expect(await first.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime)).toBeCloseTo(before, 2);
    await recordHomeAssistantEvidence(info, "actual-authenticated-profile-retirement", {realUIAuthenticatedViewer: true,
      serverIssuedSessionReplaced: true, publicViewerChanged: true, commandAcceptedBeforeSwitch: 202,
      commandDeliveredAfterSwitch: 200, actualCommandReplyHeld: true, heldMs,
      seekSeparatedByMoreThanHalfSecond: true, lateStateWrites: 0, mediaUnchanged: true});
  } catch (error) {primary = error; throw error;} finally {
    releaseCommand();
    const management = owner || (!viewerSwitched ? page : undefined);
    await cleanupHomeAssistantFixture(info, primary, [
      {operation: "close-first-document", action: async () => {if (first && !first.isClosed()) await first.close();}},
      {operation: "close-second-document", action: async () => {if (second && !second.isClosed()) await second.close();}},
      {operation: "disable-home-assistant-fixture", action: async () => {if (management) await setting(management, false);}},
      {operation: "remove-fictional-viewer", action: async () => {if (management && profile) await removeViewer(management, profile);}},
      {operation: "close-viewer-context", action: async () => {await viewerContext?.close();}},
      {operation: "close-owner-context", action: async () => {await ownerContext?.close();}},
    ]);
  }
});

test("rendered document claim failure exposes accessible Retry and recovers with a real claim", {tag: ["@smoke", "@routed-fault"]}, async ({page}, info) => {
  let first: Page | undefined, second: Page | undefined, failing = true;
  try {
    const docs = await openDocuments(page, document => document.route("**/home-assistant/players/claims", route => failing
      ? route.fulfill({status: 503}) : route.continue()), {initialClaimsRequired: false});
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
