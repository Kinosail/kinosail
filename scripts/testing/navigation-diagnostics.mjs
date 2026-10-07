// Bounded transport metadata for disposable browser fixture failures.
// Queries, credentials, headers, bodies and unknown paths never enter receipts.
export function navigationDiagnostics(page, baseURL) {
  const pending = new Map(), failed = [], mainFrameResponses = [], started = performance.now();
  let origin;
  try {
    if (typeof baseURL === "string" && baseURL.length <= 2048) {
      const url = new URL(baseURL);
      if (["http:", "https:"].includes(url.protocol) && !url.username && !url.password) {
        origin = url.origin;
      }
    }
  } catch {}
  const owned = value => {
    try {
      if (typeof value !== "string" || value.length > 2048) return;
      const url = new URL(value);
      if (url.origin!==origin || url.username || url.password) return;
      if (["/","/login","/account","/settings"].includes(url.pathname) ||
        /^\/(?:watch|subtitles\/inspect)\/[a-f0-9]{16}$/.test(url.pathname) ||
        /^\/static\/[a-zA-Z0-9._-]{1,180}\.(js|css|woff2|png|jpg|svg)$/.test(url.pathname)) return url;
    } catch {}
  };
  const path = value => {
    const pathname = owned(value)?.pathname;
    if (pathname?.startsWith("/watch/")) return "/watch";
    if (pathname?.startsWith("/subtitles/inspect/")) return "/subtitles/inspect";
    return pathname || "other";
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
  const response = value => {
    const request = value.request(), status = value.status();
    if (request.isNavigationRequest() && request.frame() === page.mainFrame() &&
        Number.isInteger(status) && status >= 100 && status <= 599 && mainFrameResponses.length < 20) {
      mainFrameResponses.push({path: path(value.url()), status});
    }
  };
  page.on("request",start); page.on("requestfinished",finish); page.on("requestfailed",fail);page.on("response",response);
  return {
    async snapshot(error) {
      let timer;
      const state = await Promise.race([
        Promise.resolve().then(()=>page.evaluate(() => ({readyState:document.readyState,libraryMarker:Boolean(document.querySelector("#main[data-view=library] #subtitle-content"))}))).catch(()=>({})),
        new Promise(resolve=>{timer=setTimeout(()=>resolve({}),500);}),
      ]);
      clearTimeout(timer);
      const readyState=state.readyState;
      const current=owned(page.url()),views=current?.searchParams.getAll("view"),view=views?.length===1?views[0]:undefined;
      return {path:path(page.url()),identity:page.url()==="about:blank"?"blank":current?"owned":"other",view:["library","movies"].includes(view)?view:"other",
        libraryMarker:Boolean(current&&state.libraryMarker),readyState:["loading","interactive","complete"].includes(readyState)?readyState:"unavailable",
        pending:[...pending.values()],failed:[...failed],mainFrameResponses:[...mainFrameResponses],redirectCount:mainFrameResponses.filter(value=>[301,302,303,307,308].includes(value.status)).length,
        elapsedMs:Math.min(600000,Math.max(0,Math.round(performance.now()-started))),errorCategory:diagnosticErrorCategory(error)};
    },
    stop() {page.off("request",start);page.off("requestfinished",finish);page.off("requestfailed",fail);page.off("response",response);},
  };
}
function diagnosticErrorCategory(error) {
  if (!error) return "none";
  if (error.name === "TimeoutError") return "timeout";
  if (typeof error.message !== "string" || error.message.length > 8192) return "other";
  const categories = [
    ["certificate", ["ERR_CERT_AUTHORITY_INVALID", "SEC_ERROR_UNKNOWN_ISSUER"]],
    ["refused", ["ERR_CONNECTION_REFUSED", "NS_ERROR_CONNECTION_REFUSED"]],
    ["interrupted", ["ERR_ABORTED", "NS_BINDING_ABORTED"]],
    ["closed", ["Target page, context or browser has been closed"]],
  ];
  for (const [category, patterns] of categories) {
    if (patterns.some(value => error.message.includes(value))) return category;
  }
  return "other";
}
