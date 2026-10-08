import {expect, test, type Page} from "@playwright/test";
import {configureTestInstance} from "./test-instance-helpers";
import {openDocuments, closeDocuments} from "./home-assistant-document-helpers";

configureTestInstance();

test("real simultaneous browser documents publish distinct Home Assistant targets", {tag: "@smoke"}, async ({page}, info) => {
  let first: Page | undefined, second: Page | undefined;
  try {
    const documents = await openDocuments(page);
    ({first, second} = documents);
    await expect.poll(async () => (await documents.live()).length).toBe(2);
    const observed = await documents.live();
    expect(new Set(observed.map(target => target.id)).size).toBe(2);
    expect(new Set(observed.map(target => target.itemId)).size).toBe(2);
    await info.attach("actual-document-targets", {body: JSON.stringify({distinctTargets: 2,
      distinctItems: 2, fixtureSHA256: "9dbd85e7863d921e209978c8349992e5f8505b0d7ffa468c37b8caf97af3f0a4"}), contentType: "application/json"});
  } finally {await closeDocuments(page, [first, second]);}
});

test("real public Home Assistant seek affects only its addressed browser document", {tag: "@smoke"}, async ({page}, info) => {
  let first: Page | undefined, second: Page | undefined;
  try {
    const documents = await openDocuments(page);
    ({first, second} = documents);
    await expect.poll(async () => (await documents.live()).length).toBe(2);
    const addressed = (await documents.live()).find(target => target.itemId === documents.ids[0])!;
    const sibling = (await documents.live()).find(target => target.itemId === documents.ids[1])!;
    const duration = await first.locator("video").evaluate((media: HTMLVideoElement) => media.duration);
    expect(Number.isFinite(duration) && duration > 2).toBe(true);
    const before = await first.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime);
    const position = before < duration / 2 ? duration * .75 : duration * .25;
    expect(Math.abs(position - before)).toBeGreaterThan(.5);
    const untouched = await second.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime);
    expect(await page.evaluate(async ({id, position}) => {
      const csrf = document.querySelector<HTMLMetaElement>('meta[name="kinosail-csrf"]')?.content || "";
      return (await fetch(`/api/v1/home-assistant/players/${encodeURIComponent(id)}/commands`, {method: "POST",
        headers: {"Content-Type": "application/json", "X-Kinosail-CSRF": csrf},
        body: JSON.stringify({command: "seek", position})})).status;
    }, {id: addressed.id, position})).toBe(202);
    const siblingPoll = second.waitForResponse(response => new URL(response.url()).pathname ===
      `/api/v1/home-assistant/players/${sibling.id}` && response.request().method() === "PUT" && response.status() === 200);
    void siblingPoll.catch(() => {});
    await expect.poll(() => first!.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime)).toBeCloseTo(position, 1);
    expect((await (await siblingPoll).json()).command).toBeNull();
    expect(await second.locator("video").evaluate((media: HTMLVideoElement) => media.currentTime)).toBeCloseTo(untouched, 2);
    await info.attach("actual-addressed-command", {body: JSON.stringify({command: "seek", position,
      addressedOnly: true, bothDocumentsPaused: true}), contentType: "application/json"});
  } finally {await closeDocuments(page, [first, second]);}
});

test("real Home Assistant document reload retains its target without storing claim authority", {tag: "@smoke"}, async ({page}, info) => {
  let first: Page | undefined, second: Page | undefined;
  try {
    const documents = await openDocuments(page);
    ({first, second} = documents);
    await expect.poll(async () => (await documents.live()).length).toBe(2);
    const before = (await documents.live()).find(target => target.itemId === documents.ids[0])!.id;
    const freshAuthority = first.waitForResponse(response => new URL(response.url()).pathname ===
      "/api/v1/home-assistant/players/claims" && response.request().method() === "POST" && response.status() === 201,
      {timeout: 40_000});
    void freshAuthority.catch(() => {});
    await first.reload();
    const renewed = await (await freshAuthority).json();
    expect(renewed.id).toBe(before);
    expect(typeof renewed.claim === "string" && renewed.claim.length >= 20).toBe(true);
    const freshClaim = renewed.claim;
    documents.claims.add(freshClaim);
    await first.waitForResponse(async response => new URL(response.url()).pathname ===
      `/api/v1/home-assistant/players/${before}` && response.request().method() === "PUT" && response.status() === 200 &&
      (await response.request().allHeaders())["x-kinosail-player-claim"] === freshClaim);
    await expect.poll(async () => (await documents.live()).find(target => target.itemId === documents.ids[0])?.id, {timeout: 40_000}).toBe(before);
    await expect.poll(() => first!.locator("video").evaluate((media: HTMLVideoElement) => media.readyState)).toBeGreaterThanOrEqual(2);
    await expect.poll(async () => (await documents.live()).length).toBe(2);
    const storage = await first.evaluate(claims => {
      const profile = document.body.dataset.viewerProfile || "";
      const key = `kinosail-home-assistant-document:${profile}`;
      const values = [sessionStorage, localStorage].flatMap(store => Object.keys(store).map(name => store.getItem(name) || ""));
      return {candidate: sessionStorage.getItem(key), claimPersisted: claims.some(claim => values.some(value => value.includes(claim)))};
    }, [...documents.claims]);
    expect(storage.candidate).toBe(before);
    expect(storage.claimPersisted).toBe(false);
    await info.attach("actual-document-reload", {body: JSON.stringify({targetRetained: true,
      distinctTargets: 2, claimPersisted: false}), contentType: "application/json"});
  } finally {await closeDocuments(page, [first, second]);}
});
