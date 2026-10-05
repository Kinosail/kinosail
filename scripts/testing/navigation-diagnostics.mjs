// Bounded transport metadata for disposable browser fixture failures.
// Queries, credentials, headers, bodies and unknown paths never enter receipts.
export function navigationDiagnostics(page) {
  const pending = new Map(), failed = [];
  const path = value => {
    try {
      if (typeof value !== "string" || value.length > 2048) return "other";
      const url = new URL(value);
      if (!["http:","https:"].includes(url.protocol) || url.username || url.password || !["localhost","127.0.0.1"].includes(url.hostname)) return "other";
      return ["/","/login","/account","/settings"].includes(url.pathname) ||
        /^\/static\/[a-zA-Z0-9._-]{1,180}\.(js|css|woff2|png|jpg|svg)$/.test(url.pathname) ? url.pathname : "other";
    } catch {return "other";}
  };
  const metadata = request => ({
    path:path(request.url()),
    type:["document","stylesheet","script","image","font","media","fetch","xhr","other"].includes(request.resourceType()) ? request.resourceType() : "other",
  });
  const start = request => {
    if (pending.size === 20) pending.delete(pending.keys().next().value);
    pending.set(request, metadata(request));
  };
  const finish = request => pending.delete(request);
  const fail = request => {
    finish(request);
    if (failed.length < 20) failed.push(metadata(request));
  };
  page.on("request",start); page.on("requestfinished",finish); page.on("requestfailed",fail);
  return {
    async snapshot() {
      let timer;
      const state = await Promise.race([
        Promise.resolve().then(()=>page.evaluate(() => document.readyState)).catch(()=>"unavailable"),
        new Promise(resolve=>{timer=setTimeout(()=>resolve("unavailable"),500);}),
      ]);
      clearTimeout(timer);
      return {path:path(page.url()), readyState:["loading","interactive","complete"].includes(state) ? state : "unavailable", pending:[...pending.values()], failed:[...failed]};
    },
    stop() {page.off("request",start); page.off("requestfinished",finish); page.off("requestfailed",fail);},
  };
}
