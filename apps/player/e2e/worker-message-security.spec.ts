import { expect, test } from "@playwright/test";
import { createServer } from "node:http";
import type { AddressInfo } from "node:net";
import { readStaticSource } from "./static-sources";

const mediaSource = await readStaticSource(["../../../packages/webassets/static/offline-shell.js"]);

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
