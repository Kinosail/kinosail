import { expect, test } from "@playwright/test";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { readStaticSource } from "./static-sources";

const workerSources = await readStaticSource([
  "../../../packages/webassets/static/downloads-storage.js",
  "../../../packages/webassets/static/downloads-integrity.js",
]);
const mediaSource = await readStaticSource(["../../../packages/webassets/static/offline-media.js"]);

test("cross-origin Window messages cannot reach dedicated worker channels", async ({ page }) => {
  await page.route("https://kinosail.test/", route => route.fulfill({contentType:"text/html",body:"<!doctype html><title>Worker security</title>"}));
  await page.route("https://other.test/", route => route.fulfill({contentType:"text/html",body:`<script>
    for (const type of ['update','hex','write','close']) parent.postMessage({id:0,type,value:{offset:0,data:new TextEncoder().encode('bad').buffer}}, '*');
  </script>`}));
  await page.goto("https://kinosail.test/");
  const result = await page.evaluate(async source => {
    const api = new Function(source + ";return {streamingOfflineDigest,offlineStorageWorkerScope}")();
    const digest = api.streamingOfflineDigest();
    // Keep the browser-owned channel and production handler real. The in-memory
    // filesystem isolates message security from browser OPFS availability.
    const storageFixture = `let bytes=new Uint8Array(3),closed=false;
      const handle={flush(){},close(){closed=true},truncate(){},
        read(target){if(closed)throw Error('closed');target.set(bytes);return bytes.length;},
        write(value){if(closed)throw Error('closed');bytes.set(value);return value.length;}};
      self.FileSystemFileHandle=class {createSyncAccessHandle(){}};
      Object.defineProperty(navigator,'storage',{value:{getDirectory:async()=>({getFileHandle:async()=>({createSyncAccessHandle:async()=>handle})})}});`;
    const url = URL.createObjectURL(new Blob([storageFixture, `(${api.offlineStorageWorkerScope.toString()})()`], {type:"text/javascript"}));
    const writer = new Worker(url);
    let id = 0;
    const request = (type: string, value?: object) => new Promise<MessageEvent>(resolve => {
      writer.addEventListener("message", resolve, {once:true});
      writer.postMessage({id:id++,type,value});
    });
    try {
      await digest.update(new TextEncoder().encode("abc").buffer);
      const opened = await request("open", {jobID:"0123456789abcdef",size:3});
      if (opened.data.error || opened.data.value !== true) throw new Error("Storage worker did not open");
      await request("write", {offset:0,data:new TextEncoder().encode("abc").buffer});
      const received: string[] = [];
      await new Promise<void>(resolve => {
        const listener = (event: MessageEvent) => {
          if (event.origin !== "https://other.test") return;
          received.push(event.data.type);
          if (received.length === 4) { window.removeEventListener("message", listener); resolve(); }
        };
        window.addEventListener("message", listener);
        const frame = document.createElement("iframe");
        frame.src = "https://other.test/";
        document.body.append(frame);
      });
      const hash = await digest.hex();
      const response = await request("read", {offset:0,length:3});
      if (response.data.error) throw new Error(response.data.error);
      const bytes = new TextDecoder().decode(response.data.value);
      return {received, hash, bytes};
    } finally { digest.close(); writer.terminate(); URL.revokeObjectURL(url); }
  }, workerSources);
  expect(result.received).toEqual(["update", "hex", "write", "close"]);
  expect(result.hash).toBe("ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad");
  expect(result.bytes).toBe("abc");
});

test("real service-worker clients can select a profile and log out after source validation", async ({ page }) => {
  const source = `let stored; const offlineProfileState=async value => {if(value!==undefined)stored=value;return stored;};
    const openOfflineDatabaseConnection=()=>{};\n${mediaSource}\nregisterOfflineLifecycle('security-worker','retired',async()=>{});`;
  const server = createServer((request, response) => {
    response.writeHead(200, {"Content-Type":request.url === "/worker.js" ? "text/javascript" : "text/html", "Cache-Control":"no-store"});
    response.end(request.url === "/worker.js" ? source : "<!doctype html><title>Worker identity security</title>");
  });
  await new Promise<void>(resolve => server.listen(0, "127.0.0.1", resolve));
  try {
    await page.goto(`http://127.0.0.1:${(server.address() as AddressInfo).port}/`);
    const profiles = await page.evaluate(async () => {
      await navigator.serviceWorker.register("/worker.js");
      await navigator.serviceWorker.ready;
      if (!navigator.serviceWorker.controller) await new Promise<void>(resolve => navigator.serviceWorker.addEventListener("controllerchange", () => resolve(), {once:true}));
      const send = (data: object) => new Promise<string>(resolve => {
        const listener = (event: MessageEvent) => {
          if (event.origin !== location.origin || event.source !== navigator.serviceWorker.controller || event.data.type !== "offline-profile") return;
          navigator.serviceWorker.removeEventListener("message", listener);
          resolve(event.data.profile);
        };
        navigator.serviceWorker.addEventListener("message", listener);
        navigator.serviceWorker.controller!.postMessage(data);
      });
      const revision = Date.now();
      return [await send({type:"profile",profile:"viewer",revision}), await send({type:"logout",profile:"",revision:revision+1})];
    });
    expect(profiles).toEqual(["viewer", ""]);
  } finally {
    await page.close(); server.closeAllConnections();
    await new Promise<void>((resolve, reject) => server.close(error => error ? reject(error) : resolve()));
  }
});
