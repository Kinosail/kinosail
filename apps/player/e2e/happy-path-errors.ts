import type {Page, Request} from "@playwright/test";

type Failure = {url: string; document: number; time: number; method: string; resource: string; failure: string; status: number | null; redirected: boolean};
type Navigation = {document: number; time: number; committed: number; from: string; to: string; method: string; redirectMethod: string; status: number; redirectStatus: number; newDocument: boolean; timeOrigin?: number};
type PageError = {document: number; documentURL: string; time: number; name: string; message: string; stack: string};
type WatchedAction = {document: number; time: number; item: string; timeOrigin: number};
export type PlaybackErrorEvidence = PageError & {browser: string; origin: string; action?: WatchedAction; failures: Failure[]; navigations: Navigation[]};

// WebKit can report a navigation-cancelled same-origin XHR as an access error.
// Require the original failed request and committed watched redirect; text alone is insufficient.
export function cancelledWatchedHLS(value: PlaybackErrorEvidence): boolean {
  if (value.browser !== "webkit" || value.stack.length > 8192 || value.failures.length > 1024 || value.navigations.length > 1024) return false;
  const match = /^XMLHttpRequest cannot load (https?:\/\/\S+) due to access control checks\.\n\s+at unknown \((https?:\/\/\S+):\d+:\d+\)/.exec(value.stack);
  if (!match || !value.message.endsWith(" due to access control checks.")) return false;
  try {
    const resource = new URL(match[1]), script = new URL(match[2]);
    const item = /^\/hls\/([a-f0-9]{16})\/p\/[a-zA-Z0-9-]+\/\d{3,4}p\/segment-\d{5}\.m4s$/.exec(resource.pathname)?.[1];
    if (!item || !value.action || value.action.document !== value.document || value.action.item !== item ||
        !Number.isFinite(value.action.time) || value.action.time > value.time || value.time - value.action.time > 2000 ||
        resource.origin !== value.origin || script.origin !== value.origin || script.pathname !== "/static/hls.min.js" ||
        resource.username || resource.password || resource.hash || value.name !== `XMLHttpRequest cannot load ${resource.protocol.slice(0, -1)}` ||
        !/^\?playbackSession=[a-f0-9]{24}$/.test(resource.search) || value.documentURL !== `${value.origin}/watch/${item}`) return false;
    return value.failures.some(failure => failure.url === resource.href && failure.document === value.document &&
      failure.method === "GET" && failure.resource === "xhr" && failure.failure === "Load request cancelled" && failure.status === null && !failure.redirected &&
      Number.isFinite(failure.time) && failure.time >= value.action!.time && failure.time - value.action!.time <= 2000 && Math.abs(failure.time - value.time) <= 1000) &&
      value.navigations.some(navigation => navigation.document === value.document && navigation.from === `${value.origin}/watched/${item}` &&
        navigation.to === value.documentURL && navigation.method === "GET" && navigation.redirectMethod === "POST" &&
        navigation.status === 200 && navigation.redirectStatus === 303 && navigation.newDocument && Number.isFinite(navigation.time) &&
        Math.abs(navigation.time - value.time) <= 1000 && navigation.committed >= Math.max(value.time, navigation.time) &&
        navigation.committed - navigation.time <= 2000);
  } catch { return false; }
}

export function captureHappyPathErrors(page: Page, browser: string) {
  let enabled = true, document = 0, overflow = false;
  let action: WatchedAction | undefined;
  const records: PageError[] = [], failures: Failure[] = [], navigations: Navigation[] = [];
  const requests = new Map<Request, Failure>(), posts = new Map<Request, {document: number; time: number; status: number}>();
  const pending: Navigation[] = [];
  let accepted: {resource: string; document: number; time: number}[] = [];
  const record = (message: string, name = "", stack = "") => {
    if (!enabled) return;
    if (records.length >= 1024) {overflow = true; return;}
    records.push({document, documentURL: page.url(), time: Date.now(), message, name, stack});
  };
  page.on("console", message => {
    if (!enabled || message.type() !== "error") return;
    // An unauthenticated login page probes its session before offering WebAuthn.
    try {
      const source = new URL(message.location().url), current = new URL(page.url());
      if (source.origin === current.origin && source.pathname === "/api/v1/me" && !source.search && current.pathname === "/login" &&
          /^Failed to load resource: the server responded with a status of 401 \([^)]*\)$/.test(message.text())) return;
    } catch { /* Messages without a URL remain failures. */ }
    record(message.text());
  });
  page.on("pageerror", error => record(error.message, error.name, error.stack ?? ""));
  page.on("request", request => {
    if (request.resourceType() !== "xhr" || request.method() !== "GET" || request.frame() !== page.mainFrame() ||
        !new URL(request.url()).pathname.startsWith("/hls/")) return;
    if (requests.size >= 1024) {overflow = true; return;}
    requests.set(request, {url: request.url(), document, time: 0, method: request.method(), resource: request.resourceType(), failure: "", status: null, redirected: Boolean(request.redirectedFrom())});
  });
  page.on("requestfailed", request => {
    const failure = requests.get(request);
    requests.delete(request);
    if (!failure) return;
    failure.time = Date.now();
    failure.failure = request.failure()?.errorText ?? "";
    if (failures.length < 1024) failures.push(failure);
    else overflow = true;
  });
  page.on("requestfinished", request => requests.delete(request));
  page.on("response", response => {
    const request = response.request(), failure = requests.get(request);
    if (failure) failure.status = response.status();
    if (!request.isNavigationRequest() || request.frame() !== page.mainFrame()) return;
    if (request.method() === "POST" && response.status() === 303 && /^\/watched\/[a-f0-9]{16}$/.test(new URL(request.url()).pathname)) {
      if (posts.size < 1024) posts.set(request, {document, time: Date.now(), status: response.status()});
      else overflow = true;
    }
    const previous = request.redirectedFrom(), post = previous && posts.get(previous);
    if (!post || !previous) return;
    posts.delete(previous);
    if (pending.length < 1024) pending.push({...post, committed: 0, newDocument: false, from: previous.url(), to: request.url(),
      method: request.method(), redirectMethod: previous.method(), status: response.status(), redirectStatus: post.status});
    else overflow = true;
  });
  page.on("framenavigated", frame => {
    if (frame !== page.mainFrame()) return;
    for (const navigation of pending.splice(0)) {
      if (navigation.document !== document || navigation.to !== frame.url()) continue;
      navigation.committed = Date.now();
      if (navigations.length < 1024) navigations.push(navigation);
      else overflow = true;
    }
    document++;
  });
  return {
    capture: (capture: boolean) => {enabled = capture;},
    beginWatched: async () => {
      const previous = await page.evaluate(() => ({url: location.href, timeOrigin: performance.timeOrigin}));
      action = {document, time: Date.now(), item: /^\/watch\/([a-f0-9]{16})$/.exec(new URL(previous.url).pathname)?.[1] ?? "", timeOrigin: previous.timeOrigin};
    },
    finishWatched: async () => {
      const current = await page.evaluate(() => ({url: location.href, timeOrigin: performance.timeOrigin}));
      for (const navigation of navigations) {
        if (action && document === action.document + 1 && navigation.document === action.document && navigation.to === current.url &&
            Number.isFinite(current.timeOrigin) && current.timeOrigin > action.timeOrigin) {
          navigation.newDocument = true;
          navigation.timeOrigin = current.timeOrigin;
        }
      }
    },
    errors: () => {
      const remaining = [...failures], unexpected: string[] = [];
      accepted = [];
      for (const error of records) {
        const origin = new URL(error.documentURL).origin;
        const index = remaining.findIndex(failure => cancelledWatchedHLS({...error, origin, browser, action, failures: [failure], navigations}));
        if (index < 0) unexpected.push(error.message);
        else {
          const [failure] = remaining.splice(index, 1);
          accepted.push({resource: new URL(failure.url).pathname, document: failure.document, time: failure.time});
        }
      }
      if (overflow) unexpected.push("Happy path error evidence exceeded its bound");
      return unexpected;
    },
    cancellations: () => accepted,
    evidence: () => records.map(error => ({...error, browser, action, origin: new URL(error.documentURL).origin, failures, navigations, pendingRequests: [...requests.values()]})),
  };
}
