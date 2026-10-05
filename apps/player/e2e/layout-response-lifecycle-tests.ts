import {createServer} from "node:http";
import {expect, test} from "@playwright/test";
import {layoutResponseHandler} from "../../../scripts/testing/layout-stability-routing.mjs";

test("Layout routing preserves decoded browser-owned images and delays real network bytes", async ({browser}) => {
  let networkImages = 0;
  const svg = '<svg xmlns="http://www.w3.org/2000/svg" width="64" height="32"><rect width="64" height="32" fill="green"/></svg>';
  const server = createServer((request,response) => {
    if (request.url === "/real.svg") {networkImages++;response.setHeader("Content-Type","image/svg+xml");response.end(svg);return;}
    response.setHeader("Content-Type","text/html");
    response.end(`<script>for(const src of [URL.createObjectURL(new Blob([${JSON.stringify(svg)}],{type:"image/svg+xml"})),"data:image/svg+xml,"+encodeURIComponent(${JSON.stringify(svg)}),"/real.svg"]){const img=new Image();img.src=src;document.documentElement.append(img);}</script>`);
  });
  await new Promise<void>(resolve=>server.listen(0,"127.0.0.1",resolve));
  const address=server.address();
  if(!address || typeof address==="string")throw new Error("Missing disposable image listener");
  const context=await browser.newContext({ignoreHTTPSErrors:false}),page=await context.newPage(),errors:Error[]=[];
  const handler=layoutResponseHandler(context,"default");
  await page.route("**/*",async route=>{try{await handler(route);}catch(error){errors.push(error as Error);await route.abort();}});
  try {
    await page.goto(`http://127.0.0.1:${address.port}`,{waitUntil:"domcontentloaded"});
    await expect.poll(()=>page.evaluate(()=>[...document.images].map(image=>image.naturalWidth))).toEqual([64,64,64]);
    expect(errors).toEqual([]);expect(networkImages).toBe(1);
  } finally {await context.close();await new Promise<void>(resolve=>server.close(()=>resolve()));}
});

// Missing/malformed/unknown/oversized route URLs must fail before any transport
// side effect; these invalid values cannot be produced by a native browser URL.
test("Layout routing rejects invalid URL inputs before fetching or continuing", async () => {
  for (const url of [undefined,null,1,"http://[","ftp://localhost/image.svg","javascript:alert(1)","file:///private/image.svg","http://localhost/"+"x".repeat(1048576)]) {
    let effects=0;
    const handler=layoutResponseHandler({once() {}} as any,"default");
    const route={request:()=>({url:()=>url,resourceType:()=>"image"}),fetch:()=>{effects++;return Promise.resolve({});},continue:()=>{effects++;return Promise.resolve();}};
    await expect(handler(route as any)).rejects.toThrow();expect(effects).toBe(0);
  }
});

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
