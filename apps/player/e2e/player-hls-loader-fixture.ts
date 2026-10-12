import type { Page } from "@playwright/test";

export async function installPendingHlsLoader(page: Page) {
  await page.evaluate(() => {
    const originalAppend = document.head.append.bind(document.head);
    document.head.append = ((...nodes: (Node | string)[]) => {
      const script = nodes.find((node): node is HTMLScriptElement => node instanceof HTMLScriptElement && node.src.includes("/static/hls.min.js"));
      if (!script) return originalAppend(...nodes);
      Object.assign(window, {hlsSources: [], releaseHlsLoader: () => {
        class TestHls {
          static DefaultConfig = {bufferController: class { onBufferAppending() {} }};
          static Events = {MANIFEST_PARSED: "manifest", LEVEL_SWITCHED: "level", ERROR: "error"};
          static ErrorTypes = {NETWORK_ERROR: "network", MEDIA_ERROR: "media"};
          static isSupported() { return true; }
          autoLevelEnabled = true; levels = [];
          on() {} loadSource(source: string) { (window as Window & {hlsSources: string[]}).hlsSources.push(source); } destroy() {}
          attachMedia(media: HTMLVideoElement | {media: HTMLVideoElement}) { ("media" in media ? media.media : media).src = "blob:stream"; }
        }
        Object.assign(window, {Hls: TestHls});
        script.onload?.(new Event("load"));
      }});
      return script;
    }) as typeof document.head.append;
  });
}
