import {expect, test} from "@playwright/test";
import {createServer} from "node:http";
import {cancelledWatchedHLS, captureHappyPathErrors, type PlaybackErrorEvidence} from "./happy-path-errors";

const origin = "https://localhost:32768";
const item = "ba4a8ce10a7d7485";
const url = `${origin}/hls/${item}/p/t-a0-s0-none-t0-b0-z1280x720-o2000/432p/segment-00001.m4s?playbackSession=9b91da4a4a7cfb1490f40e29`;

function evidence(): PlaybackErrorEvidence {
  return {
    browser: "webkit", origin, document: 4, documentURL: `${origin}/watch/${item}`, time: 146264,
    action: {document: 4, time: 145829, item, timeOrigin: 1000},
    name: "XMLHttpRequest cannot load https", message: "/localhost:32768/hls/segment due to access control checks.",
    stack: `XMLHttpRequest cannot load ${url} due to access control checks.\n    at unknown (${origin}/static/hls.min.js?v=1.7.1:3:565997)`,
    failures: [{url, document: 4, time: 146138, method: "GET", resource: "xhr", failure: "Load request cancelled", status: null, redirected: false}],
    navigations: [{document: 4, time: 146277, committed: 146398, from: `${origin}/watched/${item}`, to: `${origin}/watch/${item}`,
      method: "GET", redirectMethod: "POST", status: 200, redirectStatus: 303, newDocument: true}],
  };
}

const controls: Record<string, (value: PlaybackErrorEvidence) => void> = {
  "observed cancellation": () => {},
  "other browser": value => {value.browser = "firefox";},
  "ordinary script error": value => {value.name = "TypeError";},
  "different message": value => {value.stack = value.stack.replace("access control checks", "connection refused");},
  "missing stack": value => {value.stack = "";},
  "oversized stack": value => {value.stack += "x".repeat(8192);},
  "different origin": value => {value.origin = "https://elsewhere.example";},
  "different script": value => {value.stack = value.stack.replace("/static/hls.min.js", "/static/player.js");},
  "missing cancellation": value => {value.failures = [];},
  "network failure": value => {value.failures[0].failure = "Connection refused";},
  "different request": value => {value.failures[0].url = url.replace("00001", "00002");},
  "different method": value => {value.failures[0].method = "POST";},
  "different resource": value => {value.failures[0].resource = "fetch";},
  "different failed document": value => {value.failures[0].document++;},
  "redirected fragment": value => {value.failures[0].redirected = true;},
  "before watched action": value => {value.action!.time = value.time + 1;},
  "cancelled before watched action": value => {value.action!.time = value.failures[0].time + 1;},
  "missing watched action": value => {value.action = undefined;},
  "different action document": value => {value.action!.document++;},
  "different action item": value => {value.action!.item = "0000000000000000";},
  "stale watched action": value => {value.action!.time -= 3000;},
  "successful HTTP response": value => {value.failures[0].status = 200;},
  "HTTP denial": value => {value.failures[0].status = 403;},
  "stale cancellation": value => {value.failures[0].time -= 2000;},
  "late action cancellation": value => {value.action!.time = value.time - 1800; value.failures[0].time = value.time + 400;},
  "missing committed navigation": value => {value.navigations = [];},
  "different departing document": value => {value.navigations[0].document++;},
  "different watched item": value => {value.navigations[0].from = `${origin}/watched/0000000000000000`;},
  "different destination": value => {value.navigations[0].to = `${origin}/`;},
  "different error document": value => {value.documentURL = `${origin}/`;},
  "failed redirect": value => {value.navigations[0].redirectStatus = 403;},
  "failed destination": value => {value.navigations[0].status = 500;},
  "wrong navigation method": value => {value.navigations[0].redirectMethod = "GET";},
  "stale navigation": value => {value.navigations[0].time -= 2000;},
  "uncommitted navigation": value => {value.navigations[0].committed = 0;},
  "same document": value => {value.navigations[0].newDocument = false;},
  "late commit": value => {value.navigations[0].committed += 10_000;},
  "unknown query": value => {value.stack = value.stack.replace("playbackSession=", "unknown=");},
  "conflicting query": value => {value.stack = value.stack.replace(" due to", "&playbackSession=other due to");},
  "malformed session": value => {value.stack = value.stack.replace("9b91da4a4a7cfb1490f40e29", "invalid");},
  "non-segment resource": value => {value.stack = value.stack.replace("segment-00001.m4s", "index.m3u8");},
};

for (const [scenario, change] of Object.entries(controls)) {
  test(`happy path preserves resource failures: ${scenario} @smoke`, async ({}, info) => {
    const value = evidence();
    change(value);
    const before = JSON.stringify(value);
    expect(cancelledWatchedHLS(value)).toBe(scenario === "observed cancellation");
    expect(JSON.stringify(value)).toBe(before);
    await info.attach("resource-error-classification", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
      scenario, evidence: "Exact retained hosted WebKit cancellation signature with one changed prerequisite", result: "passed"}), contentType: "application/json"});
  });
}

for (const scenario of ["cancelled", "denied", "redirected", "different-item", "failed-navigation", "unrelated-error", "duplicate-error"]) {
  test(`happy path records actual HTTP navigation and XHR evidence: ${scenario} @smoke`, async ({page, browserName}, info) => {
    let submitted = false, started = false;
    const nextItem = scenario === "different-item" ? "0000000000000000" : item;
    const server = createServer((request, response) => {
      const path = new URL(request.url!, "http://localhost").pathname;
      if (path === "/favicon.ico") {response.writeHead(204); response.end(); return;}
      if (path.startsWith("/hls/")) {
        if (scenario === "redirected" && path.endsWith("segment-00001.m4s")) {
          response.writeHead(302, {Location: new URL(url).pathname.replace("00001", "00002") + new URL(url).search}); response.end(); return;
        }
        started = true;
        if (scenario === "denied") {response.writeHead(403); response.end("Denied");}
        return; // Hold a real XHR until the native form navigation cancels it.
      }
      if (request.method === "POST" && path === `/watched/${nextItem}`) {
        submitted = true;
        response.writeHead(scenario === "failed-navigation" ? 500 : 303, {Location: `/watch/${nextItem}`});
        response.end("Watched submission");
        return;
      }
      if (path === "/static/hls.min.js") {
        response.setHeader("Content-Type", "text/javascript");
        response.end(`const source=location.origin+${JSON.stringify(new URL(url).pathname + new URL(url).search)};
          const xhr=new XMLHttpRequest(); xhr.open('GET',source); xhr.send();
          const fail=()=>{const error=new Error('/localhost/hls/segment due to access control checks.');
            error.name=${JSON.stringify(scenario === "unrelated-error" ? "TypeError" : "XMLHttpRequest cannot load http")};
            error.stack='XMLHttpRequest cannot load '+source+' due to access control checks.\\n    at unknown ('+location.origin+'/static/hls.min.js:1:1)'; throw error;};
          document.querySelector('form').addEventListener('submit',fail);
          ${scenario === "duplicate-error" ? "document.querySelector('form').addEventListener('submit',()=>fail());" : ""}`);
        return;
      }
      if (path.startsWith("/watch/")) {
        response.setHeader("Content-Type", "text/html");
        response.end(`<form method="post" action="/watched/${nextItem}"><button>${submitted ? "Mark unwatched" : "Mark watched"}</button></form>${submitted ? "" : '<script src="/static/hls.min.js"></script>'}`);
        return;
      }
      response.writeHead(404); response.end();
    });
    await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
    const address = server.address();
    if (!address || typeof address === "string") throw new Error("HTTP control did not bind");
    const monitor = captureHappyPathErrors(page, browserName);
    try {
      if (scenario === "redirected") {
        const fragment = new URL(url);
        const probe = await fetch(`http://127.0.0.1:${address.port}//untrusted.invalid${fragment.pathname}${fragment.search}`, {redirect: "manual"});
        expect(probe.status).toBe(302);
        expect(probe.headers.get("Location")).toBe(fragment.pathname.replace("00001", "00002") + fragment.search);
      }
      await page.goto(`http://127.0.0.1:${address.port}/watch/${item}`);
      await expect.poll(() => started).toBe(true);
      await monitor.beginWatched();
      await page.getByRole("button", {name: "Mark watched", exact: true}).click();
      if (scenario === "failed-navigation") await expect(page.getByText("Watched submission")).toBeVisible();
      else await expect(page.getByRole("button", {name: "Mark unwatched", exact: true})).toBeVisible();
      await monitor.finishWatched();
      await info.attach("raw-error-evidence", {body: JSON.stringify(monitor.evidence()), contentType: "application/json"});
      const observed = monitor.evidence();
      expect(observed.length).toBeGreaterThan(0);
      if (scenario === "failed-navigation") expect(observed[0].navigations).toHaveLength(0);
      else expect(observed[0].navigations).toMatchObject([{status: 200, redirectStatus: 303, method: "GET", redirectMethod: "POST", newDocument: true}]);
      if (scenario === "redirected") expect([...observed[0].failures, ...observed[0].pendingRequests].some(failure => failure.redirected)).toBe(true);
      // A deliberately thrown error is not the engine's native HLS XHR exception.
      // Even an actual cancellation and successful navigation must not excuse it.
      if (["denied", "failed-navigation"].includes(scenario)) await expect.poll(() => monitor.errors().length).toBeGreaterThan(0);
      else await expect.poll(() => monitor.errors().length).toBe(scenario === "duplicate-error" ? 2 : 1);
      expect(monitor.cancellations()).toHaveLength(0);
      await info.attach("actual-http-error-control", {body: JSON.stringify({revision: process.env.KINOSAIL_TEST_REVISION,
        browser: browserName, scenario, submitted, started, cancellations: monitor.cancellations(), errors: monitor.errors(),
        boundary: "Real HTTP/XHR/form navigation; deliberately thrown recorded browser error signature; no media decoder proof", result: "passed"}), contentType: "application/json"});
    } finally {
      server.closeAllConnections();
      await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
    }
  });
}
