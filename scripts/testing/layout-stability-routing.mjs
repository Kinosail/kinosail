// Delay real bytes for layout measurements; lifecycle failures remain observable.
export function layoutResponseHandler(context, variant, recordFailure) {
  let closed = false;
  context.once("close", () => { closed = true; });
  return async route => {
    try {
      const request = route.request(), value = request.url();
      if (typeof value !== "string" || value.length > 1048576) throw new Error("Invalid layout resource URL");
      const url = new URL(value);
      if (["blob:", "data:"].includes(url.protocol)) { await route.continue(); return; }
      if (!["http:", "https:"].includes(url.protocol)) throw new Error("Unsupported layout resource protocol");
      if ((variant === "slow-css" && request.resourceType() === "stylesheet") || url.pathname.endsWith(".woff2") ||
          (url.pathname.endsWith(".js") && !url.pathname.endsWith("/theme.js")) ||
          /\/api\/v1\/subtitle-library\/[^/]+\/inspect/.test(url.pathname) || request.resourceType() === "image") {
        const response = await route.fetch();
        await new Promise(resolve => setTimeout(resolve, variant === "slow-css" && request.resourceType() === "script" ? 2400 : 1200));
        await route.fulfill({response});
      } else await route.continue();
    } catch (error) {
      if (!closed) {
        await route.abort("failed").catch(() => {});
        await recordFailure?.(error, route).catch(() => {});
        throw error;
      }
    }
  };
}
