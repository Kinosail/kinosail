// Bounded transport metadata for disposable browser fixture failures.
// Queries, credentials, headers, bodies and unknown paths never enter receipts.
export function navigationDiagnostics(page, baseURL) {
  const pending = new Map(), failed = [], mainFrameResponses = [], lifecycle = [], started = performance.now();
  const mainFrameRequests = [], requestIdentities = new WeakMap();
  let requestSequence = 0;
  const elapsed = () => Math.min(600000, Math.max(0, Math.round(performance.now() - started)));
  const documents = [], counts = {requests: 0, responses: 0, finished: 0, failed: 0, pageErrors: 0, workers: 0, crashes: 0};
  const increment = kind => {counts[kind] = Math.min(100000, counts[kind] + 1);};
  let documentListener;
  let navigationAttempt = 0;
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
      if (["/","/login","/setup","/account","/settings"].includes(url.pathname) ||
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
  const route = value => {
    if (value === "about:blank") return {path: "blank", view: "other"};
    let candidate = value;
    try { if (typeof value === "string" && baseURL) candidate = new URL(value, baseURL).href; } catch {}
    const url = owned(candidate);
    const views = url?.searchParams.getAll("view");
    return {path: path(candidate), view: views?.length === 1 && ["library", "movies"].includes(views[0]) ? views[0] : "other"};
  };
  const recordLifecycle = (kind, value, frameKind) => {
    if (lifecycle.length === 20) {
      const keepStart = lifecycle[0]?.kind === "navigation-start";
      lifecycle.splice(keepStart ? 1 : 0, 1);
    }
    lifecycle.push({timeMs: Math.min(600000, Math.max(0, Math.round(performance.now() - started))),
      attempt: navigationAttempt || undefined, kind, ...(frameKind ? {frame: frameKind} : {}), ...(value ? {route: route(value)} : {})});
  };
  const frameNavigation = frame => {
    let value = "";
    try { value = frame.url(); } catch {}
    let frameKind = "child";
    try { if (frame === page.mainFrame()) frameKind = "main"; } catch {}
    recordLifecycle("frame-navigated", value, frameKind);
  };
  const domContentLoaded = () => recordLifecycle("domcontentloaded", page.url(), "main");
  const loaded = () => recordLifecycle("load", page.url(), "main");
  const closed = () => recordLifecycle("close", page.url(), "main");
  const crashed = () => {increment("crashes"); recordLifecycle("crash", page.url(), "main");};
  const pageError = () => increment("pageErrors");
  const worker = () => increment("workers");
  const metadata = request => {
    let identity = requestIdentities.get(request);
    if (!identity) {
      identity = {requestID: requestSequence < 100000 ? ++requestSequence : "unavailable", firstSeenMs: elapsed()};
      requestIdentities.set(request, identity);
    }
    return {path: path(request.url()),
      type: ["document","stylesheet","script","image","font","media","fetch","xhr","other"].includes(request.resourceType()) ? request.resourceType() : "other",
      ...identity, timeMs: elapsed()};
  };
  const isMainNavigation = request => {
    try {return request.isNavigationRequest() && request.frame() === page.mainFrame();}
    catch {return false;}
  };
  const start = request => {
    increment("requests");
    if (pending.size === 20) pending.delete(pending.keys().next().value);
    const value = metadata(request);
    pending.set(request, value);
    if (isMainNavigation(request)) {
      if (mainFrameRequests.length === 20) mainFrameRequests.shift();
      mainFrameRequests.push(value);
    }
  };
  const finish = request => {increment("finished"); pending.delete(request);};
  const fail = request => {
    increment("failed"); pending.delete(request);
    if (failed.length < 20) failed.push(metadata(request));
  };
  const response = value => {
    increment("responses");
    const request = value.request(), status = value.status();
    if (isMainNavigation(request) &&
        Number.isInteger(status) && status >= 100 && status <= 599 && mainFrameResponses.length < 20) {
      const {requestID, firstSeenMs} = metadata(request);
      let fromServiceWorker = "unavailable";
      try {const flag = value.fromServiceWorker(); if (typeof flag === "boolean") fromServiceWorker = flag;} catch {}
      mainFrameResponses.push({path: path(value.url()), status, requestID, firstSeenMs, timeMs: elapsed(), fromServiceWorker});
    }
  };
  page.on("request",start); page.on("requestfinished",finish); page.on("requestfailed",fail);page.on("response",response);
  page.on("framenavigated",frameNavigation); page.on("domcontentloaded",domContentLoaded); page.on("load",loaded);
  page.on("close",closed); page.on("crash",crashed);
  page.on("pageerror",pageError); page.on("worker",worker);
  return {
    async observeDocument(onRecord) {
      if (documentListener) return;
      documentListener = message => {
        try {
          const text = message.text();
          if (typeof text !== "string" || text.length > 1024 || !text.startsWith("KINOSAIL_NAV_DOCUMENT ")) return;
          const raw = text.slice(22), value = JSON.parse(raw);
          const fields = ["kind", "main", "loginPath", "setupPath", "readyState", "timeOrigin", "elapsedMs", "loginForm", "setupForm"];
          const keys = [...raw.matchAll(/"([^"\\]+)"\s*:/g)].map(match => match[1]);
          if (!value || typeof value !== "object" || keys.length !== fields.length || new Set(keys).size !== fields.length ||
              keys.some(key => !fields.includes(key)) || !["start", "dcl", "load"].includes(value.kind) || value.main !== true ||
              typeof value.loginPath !== "boolean" || typeof value.loginForm !== "boolean" ||
              typeof value.setupPath !== "boolean" || typeof value.setupForm !== "boolean" ||
              value.loginPath && value.setupPath || value.loginForm && !value.loginPath || value.setupForm && !value.setupPath ||
              !["loading", "interactive", "complete"].includes(value.readyState) ||
              !Number.isFinite(value.timeOrigin) || value.timeOrigin < 0 || value.timeOrigin > 1e14 ||
              !Number.isFinite(value.elapsedMs) || value.elapsedMs < 0 || value.elapsedMs > 600000) return;
          if (documents.length === 20) documents.shift();
          // Console payloads can be emitted by any script; their claimed frame
          // identity is evidence to compare, not a trusted document identity.
          const record = {...value, source: "unverified-console"};
          documents.push(record);
          onRecord?.(record);
        } catch { /* Observation never changes document behavior. */ }
      };
      page.on("console", documentListener);
      await page.addInitScript(() => {
        const guard = Symbol.for("kinosail:e2e:navigation-document");
        if (window[guard]) return;
        window[guard] = true;
        const record = kind => console.debug("KINOSAIL_NAV_DOCUMENT " + JSON.stringify({kind,
          main: window.top === window, loginPath: location.pathname === "/login", setupPath: location.pathname === "/setup", readyState: document.readyState,
          timeOrigin: performance.timeOrigin, elapsedMs: Math.min(600000, performance.now()),
          loginForm: location.pathname === "/login" && Boolean(document.querySelector('form input[name="name"]')),
          setupForm: location.pathname === "/setup" && Boolean(document.querySelector('form input[name="name"]'))}));
        record("start");
        document.addEventListener("DOMContentLoaded", () => record("dcl"), {once: true});
        window.addEventListener("load", () => record("load"), {once: true});
      });
    },
    markNavigation(target) {
      navigationAttempt = Math.min(20, navigationAttempt + 1);
      recordLifecycle("navigation-start", target, "main");
    },
    async snapshot(error) {
      let timer;
      const state = await Promise.race([
        Promise.resolve().then(()=>page.evaluate(() => ({readyState:document.readyState,
          libraryMarker:Boolean(document.querySelector("#main[data-view=library] #subtitle-content")),
          loginForm:location.pathname==="/login"&&Boolean(document.querySelector('form input[name="name"]')),
          setupForm:location.pathname==="/setup"&&Boolean(document.querySelector('form input[name="name"]')),
          timeOrigin:performance.timeOrigin}))).catch(()=>({})),
        new Promise(resolve=>{timer=setTimeout(()=>resolve({}),500);}),
      ]);
      clearTimeout(timer);
      const readyState=state.readyState;
      const current=owned(page.url()),views=current?.searchParams.getAll("view"),view=views?.length===1?views[0]:undefined;
      return {path:path(page.url()),identity:page.url()==="about:blank"?"blank":current?"owned":"other",view:["library","movies"].includes(view)?view:"other",
        libraryMarker:Boolean(current&&state.libraryMarker),readyState:["loading","interactive","complete"].includes(readyState)?readyState:"unavailable",
        loginForm:state.loginForm===true,setupForm:state.setupForm===true,timeOrigin:Number.isFinite(state.timeOrigin)&&state.timeOrigin>=0&&state.timeOrigin<=1e14?state.timeOrigin:undefined,
        documents:[...documents],counts:{...counts},
        pending:[...pending.values()],failed:[...failed],mainFrameRequests:[...mainFrameRequests],mainFrameResponses:[...mainFrameResponses],redirectCount:mainFrameResponses.filter(value=>[301,302,303,307,308].includes(value.status)).length,
        lifecycle:[...lifecycle],
        elapsedMs:Math.min(600000,Math.max(0,Math.round(performance.now()-started))),errorCategory:diagnosticErrorCategory(error)};
    },
    stop() {page.off("request",start);page.off("requestfinished",finish);page.off("requestfailed",fail);page.off("response",response);
      page.off("framenavigated",frameNavigation);page.off("domcontentloaded",domContentLoaded);page.off("load",loaded);page.off("close",closed);page.off("crash",crashed);
      page.off("pageerror",pageError);page.off("worker",worker);if(documentListener)page.off("console",documentListener);},
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
