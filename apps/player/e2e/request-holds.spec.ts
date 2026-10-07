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
