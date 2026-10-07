import {expect, test} from "@playwright/test";
import {createServer, type Server} from "node:http";
import {holdNextMainRequest} from "./request-holds";

// The Owner journey uses HTMX/XHR, while other Library requests use fetch.
// This real HTTP fixture verifies that its fault-injection barrier observes both
// transports without changing app code or synthesizing the held response.
test.use({serviceWorkers: "block"});
let server: Server, origin: string;
let requests: string[];
test.beforeAll(async () => {
  server = createServer((request, response) => {
    const url = new URL(request.url!, "http://localhost");
    if (url.pathname === "/worker.js") {
      response.setHeader("Content-Type", "text/javascript");
      response.end(`self.addEventListener('install', event => event.waitUntil(self.skipWaiting()));
        self.addEventListener('activate', event => event.waitUntil(self.clients.claim()));
        self.addEventListener('fetch', event => event.respondWith(fetch(event.request)));`);
      return;
    }
    if (url.search) requests.push(url.searchParams.get("letter") || "other");
    response.setHeader("Content-Type", url.search ? "text/plain" : "text/html");
    response.end(url.search ? "received" : '<!doctype html><title>HTTP hold fixture</title><output></output>');
  });
  await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("owned HTTP fixture address unavailable");
  origin = `http://127.0.0.1:${address.port}`;
});
test.afterAll(async () => new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve())));
test.beforeEach(() => {requests = [];});

for (const transport of ["fetch", "xhr"] as const) {
  test(`main request hold gates the real ${transport} response until release`, async ({page}, info) => {
    await page.goto(origin);
    const held = await holdNextMainRequest(page, "letter", "B");
    try {
      await page.evaluate(transport => {
        const show = (text: string) => {document.querySelector("output")!.textContent = text;};
        if (transport === "fetch") void fetch("/?letter=B").then(response => response.text()).then(show);
        else {
          const request = new XMLHttpRequest();
          request.open("GET", "/?letter=B");
          request.setRequestHeader("HX-Request", "true");
          request.onload = () => show(request.responseText);
          request.send();
        }
      }, transport);
      await held.waitUntilStarted();
      expect(requests).toEqual([]);
      await expect(page.locator("output")).toHaveText("");
      await held.release();
      await expect(page.locator("output")).toHaveText("received");
      expect(requests).toEqual(["B"]);
      await info.attach("transport-hold", {body: JSON.stringify({transport, deliveredBeforeRelease: false,
        deliveredAfterRelease: 1, boundary: "Owned real HTTP fixture; transport support, not product journey proof."}), contentType: "application/json"});
    } finally {await held.release();}
  });
}

test("a nonmatching main request remains usable while the chosen request is held", async ({page}) => {
  await page.goto(origin);
  const held = await holdNextMainRequest(page, "letter", "B");
  try {
    const result = await page.evaluate(async () => (await fetch("/?letter=A")).text());
    expect(result).toBe("received");
    expect(requests).toEqual(["A"]);
  } finally {await held.release();}
});

test("an aborted held XHR is never sent when the gate releases", async ({page}) => {
  await page.goto(origin);
  const held = await holdNextMainRequest(page, "letter", "B");
  try {
    await page.evaluate(() => {
      const request = new XMLHttpRequest();
      request.open("GET", "/?letter=B");
      request.send();
      request.abort();
    });
    await held.waitUntilStarted();
    await held.release();
    // A completed following exchange proves the server remains usable after
    // cancellation and gives the retired request a chance to reach transport.
    expect(await page.evaluate(async () => (await fetch("/?letter=A")).text())).toBe("received");
    expect(requests).toEqual(["A"]);
  } finally {await held.release();}
});

test("a matching POST is not delayed by the GET-only gate", async ({page}) => {
  await page.goto(origin);
  const held = await holdNextMainRequest(page, "letter", "B");
  try {
    expect(await page.evaluate(async () => (await fetch("/?letter=B", {method: "POST"})).text())).toBe("received");
    expect(requests).toEqual(["B"]);
  } finally {await held.release();}
});

test("a second hold in the same document has its own started witness", async ({page}) => {
  await page.goto(origin);
  const first = await holdNextMainRequest(page, "letter", "B");
  try {
    await page.evaluate(() => {void fetch("/?letter=B").then(response => response.text()).then(text => {
      document.querySelector("output")!.textContent = text;
    });});
    await first.waitUntilStarted();
    expect(requests).toEqual([]);
    await first.release();
    await expect(page.locator("output")).toHaveText("received");
  } finally {await first.release();}
  const second = await holdNextMainRequest(page, "letter", "C");
  let started = false;
  const witness = second.waitUntilStarted().then(() => {started = true;});
  try {
    expect(await page.evaluate(async () => (await fetch("/?letter=A")).text())).toBe("received");
    expect(started).toBe(false);
    expect(requests).toEqual(["B", "A"]);
    await page.locator("output").evaluate(output => {output.textContent = "";});
    await page.evaluate(() => {void fetch("/?letter=C").then(response => response.text()).then(text => {
      document.querySelector("output")!.textContent = text;
    });});
    await witness;
    expect(requests).toEqual(["B", "A"]);
    await expect(page.locator("output")).toHaveText("");
    await second.release();
    await expect(page.locator("output")).toHaveText("received");
    expect(requests).toEqual(["B", "A", "C"]);
  } finally {await second.release(); await witness;}
});

test.describe("service-worker-controlled real HTTP transports", () => {
  test.use({serviceWorkers: "allow"});
  for (const transport of ["fetch", "xhr"] as const) {
    test(`main request hold gates controlled ${transport} until release`, async ({page}, info) => {
      await page.goto(origin);
      await page.evaluate(async () => {
        await navigator.serviceWorker.register("/worker.js");
        await navigator.serviceWorker.ready;
        if (!navigator.serviceWorker.controller) await new Promise<void>(resolve =>
          navigator.serviceWorker.addEventListener("controllerchange", () => resolve(), {once: true}));
      });
      expect(await page.evaluate(() => Boolean(navigator.serviceWorker.controller))).toBe(true);
      const held = await holdNextMainRequest(page, "letter", "B");
      try {
        await page.evaluate(transport => {
          const show = (text: string) => {document.querySelector("output")!.textContent = text;};
          if (transport === "fetch") void fetch("/?letter=B").then(response => response.text()).then(show);
          else {
            const request = new XMLHttpRequest();
            request.open("GET", "/?letter=B");
            request.setRequestHeader("HX-Request", "true");
            request.onload = () => show(request.responseText);
            request.send();
          }
        }, transport);
        await held.waitUntilStarted();
        expect(requests).toEqual([]);
        await expect(page.locator("output")).toHaveText("");
        await held.release();
        await expect(page.locator("output")).toHaveText("received");
        expect(requests).toEqual(["B"]);
        await info.attach("controlled-transport", {body: JSON.stringify({transport, workerControlled: true,
          deliveredBeforeRelease: false, deliveredAfterRelease: 1}), contentType: "application/json"});
      } finally {await held.release();}
    });
  }
});
