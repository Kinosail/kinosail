import {createServer} from "node:http";
import {expect, test} from "@playwright/test";
import {layoutResponseHandler} from "../../../scripts/testing/layout-stability-routing.mjs";

// A delayed real upstream response can fail after its owned page is discarded.
// Cancellation must not hide failures while that same context is still active.
for (const closed of [true, false]) test(`Delayed layout response preserves ${closed ? "owned context cancellation" : "active request failure"}`, async ({browser}) => {
  let release!: () => void, observed!: () => void;
  const held = new Promise<void>(resolve => { release = resolve; });
  const fetched = new Promise<void>(resolve => { observed = resolve; });
  const server = createServer(async (request, response) => {
    if (request.url === "/held") {
      observed(); await held;
      if (closed) response.end("real synthetic image bytes");
      else response.destroy();
    } else {
      response.setHeader("Content-Type", "text/html"); response.end('<img src="/held">');
    }
  });
  await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
  const address = server.address();
  if (!address || typeof address === "string") throw new Error("Disposable listener missing");
  const context = await browser.newContext({ignoreHTTPSErrors: false});
  const page = await context.newPage(), errors: Error[] = [];
  const handler = layoutResponseHandler(context, "default");
  let settled!: () => void;
  const completed = new Promise<void>(resolve => { settled = resolve; });
  await page.route("**/held", async route => {
    try { await handler(route); } catch (error) { errors.push(error as Error); } finally { settled(); }
  });
  try {
    await page.goto(`http://127.0.0.1:${address.port}`, {waitUntil: "domcontentloaded"});
    await fetched;
    if (closed) { setTimeout(release, 100); await context.close(); }
    else release();
    await completed;
    expect(errors.length).toBe(closed ? 0 : 1);
    if (!closed) expect(page.isClosed()).toBe(false);
  } finally {
    release(); await context.close();
    await new Promise<void>(resolve => server.close(() => resolve()));
  }
});
