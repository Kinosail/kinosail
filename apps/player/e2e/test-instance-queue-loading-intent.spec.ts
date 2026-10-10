import {expect, test} from "@playwright/test";
import {configureTestInstance} from "./test-instance-helpers";
import {runLoadingIntent, tracks} from "./queue-loading-intent-helpers";

configureTestInstance();
test.skip(!process.env.KINOSAIL_QUEUE_LOADING_URL, "requires the real Server media transport barrier");
test.use({serviceWorkers: "block"});
for (const command of ["seek", "stop"] as const) {
  test(`real queued track preserves ${command} arriving before actual metadata`, {tag: ["@smoke", "@network-delay"]},
    ({page}, info) => runLoadingIntent(page, info, command));
  test(`real queued saved35 track preserves ${command} arriving before actual metadata`, {tag: ["@smoke", "@network-delay"]},
    ({page}, info) => runLoadingIntent(page, info, command, "Queue Intent Long Session", 35));
}


test("media barrier rejects invalid targets without changing its state", async ({request}) => {
  const before = await (await request.get("/__queue-media")).json();
  for (const path of ["", "/media/missing", "/media/0000000000000000", "/media/" + "a".repeat(4097), "?unknown=1"]) {
    expect((await request.put("/__queue-media" + path)).status()).toBe(path ? 400 : 409);
    expect(await (await request.get("/__queue-media")).json()).toEqual(before);
  }
  expect((await request.put("/__queue-media/media/0000000000000000", {data: "invalid"})).status()).toBe(400);
  expect(await (await request.get("/__queue-media")).json()).toEqual(before);
});


test("armed media barrier rejects conflicting or malformed changes without releasing bytes", async ({page, request}) => {
  const [first, second] = await tracks(page);
  const target = `/__queue-media${second.stream}`;
  expect((await request.put(target)).status()).toBe(204);
  try {
    const before = await (await request.get("/__queue-media")).json();
    expect((await request.put(target)).status()).toBe(409);
    expect((await request.delete(`/__queue-media${first.stream}`)).status()).toBe(409);
    expect((await request.post(target)).status()).toBe(409);
    for (const path of [target + "?unknown=1", target.replace("/media/", "/m%65dia/")]) {
      expect((await request.put(path)).status()).toBe(400);
      expect(await (await request.get("/__queue-media")).json()).toEqual(before);
    }
    expect((await request.put(target, {data: "x".repeat(1025)})).status()).toBe(400);
    expect(await (await request.get("/__queue-media")).json()).toEqual(before);
  } finally { expect((await request.delete(target)).status()).toBe(204); }
});
