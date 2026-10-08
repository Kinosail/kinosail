// A stored ID is only a document reload candidate. Authority stays in memory.
const createHomeAssistantDocumentPlayer = adapter => {
  const validID = value => typeof value === "string" && /^[A-Za-z0-9_-]{1,64}$/.test(value);
  const exactKeys = (value, allowed) => value && typeof value === "object" && !Array.isArray(value) && Object.keys(value).every(key => allowed.includes(key));
  const profile = adapter.profile(), storageKey = `kinosail-home-assistant-document:${profile}`;
  const root = "/api/v1/home-assistant/players";
  let candidate = "", lease, flight, again = false, hidden = false, terminal = false;
  let generation = 0, interval, events, retryTimer, retryResolve, reconnect = false, exhausted = false;
  let storageAvailable = true, forkedCandidate = false;
  const controllers = new Set();
  const owner = () => !terminal && !hidden && adapter.current() && adapter.profile() === profile;
  const current = revision => revision === generation && owner();
  const sameSource = source => adapter.snapshot().source === source;
  try {const stored = sessionStorage.getItem(storageKey); if (validID(stored)) candidate = stored;} catch (_) {storageAvailable = false;}
  try {reconnect = ["reload", "back_forward"].includes(performance.getEntriesByType("navigation")[0]?.type);} catch (_) {}
  const diagnose = (operation, failure, requestID = "", level = "warn") => adapter.diagnose?.({operation, failure,
    level, generation, requestID: validID(requestID) ? requestID : ""});
  const status = (state, retry = false) => adapter.status?.({state, retry, storageAvailable, forkedCandidate});
  const headers = claim => {
    const token = adapter.csrf();
    return {"Content-Type": "application/json", ...(typeof token === "string" && token.length <= 128 && !/[\u0000-\u001f\u007f]/.test(token) && token ? {"X-Kinosail-CSRF": token} : {}),
      ...(claim ? {"X-Kinosail-Player-Claim": claim} : {})};
  };
  const request = async (path, method, body, claim, limit, deadline = Infinity) => {
    const controller = new AbortController();
    controllers.add(controller);
    let reader, timedOut = false, rejectAbort, response;
    const aborted = new Promise((_, reject) => rejectAbort = reject);
    const retire = () => {void reader?.cancel().catch(() => {}); rejectAbort({failure: timedOut ? "timeout" : "retired"});};
    controller.signal.addEventListener("abort", retire, {once: true});
    const remaining = Math.min(5000, Math.max(0, deadline - performance.now()));
    const timer = setTimeout(() => {timedOut = true; controller.abort();}, remaining);
    try {
      if (remaining <= 0) {timedOut = true; controller.abort();}
      response = await Promise.race([fetch(path, {method, credentials: "same-origin", cache: "no-store",
        headers: headers(claim), ...(body === undefined ? {} : {body: JSON.stringify(body)}), signal: controller.signal}), aborted]);
      const requestID = response.headers.get("X-Request-ID") || "";
      const status = response.redirected ? 401 : response.status;
      if (![200, 201].includes(status)) {
        void response.body?.cancel().catch(() => {});
        return {status, requestID, retry: response.headers.get("Retry-After")};
      }
      if (response.headers.get("Content-Type")?.split(";")[0].trim().toLowerCase() !== "application/json" || !response.body) throw {failure: "response", requestID};
      reader = response.body.getReader();
      const decoder = new TextDecoder("utf-8", {fatal: true});
      let length = 0, text = "";
      while (true) {
        const {value, done} = await Promise.race([reader.read(), aborted]);
        if (done) break;
        length += value.byteLength;
        if (length > limit) throw {failure: "response", requestID};
        text += decoder.decode(value, {stream: true});
      }
      let parsed;
      try {parsed = JSON.parse(text + decoder.decode());} catch (_) {throw {failure: "response", requestID};}
      return {status, requestID, body: parsed};
    } catch (error) {
      throw {failure: error?.failure || (response ? "response" : "network"), requestID: error?.requestID || response?.headers.get("X-Request-ID") || ""};
    } finally {
      clearTimeout(timer); controllers.delete(controller);
      controller.signal.removeEventListener("abort", retire);
      void reader?.cancel().catch(() => {});
    }
  };
  const viewer = async (revision, deadline = Infinity) => {
    const response = await request("/api/v1/me", "GET", undefined, undefined, 16384, deadline);
    if (!current(revision)) return false;
    if (response.status !== 200 || response.body?.viewer?.id !== profile) {
      diagnose("profile", response.status === 200 ? "ownership" : "authentication", response.requestID);
      stop(); terminal = true;
      return false;
    }
    return true;
  };
  const wait = delay => new Promise(resolve => {
    retryResolve = resolve;
    retryTimer = setTimeout(() => {retryTimer = undefined; retryResolve = undefined; resolve();}, delay);
  });
  const acquire = async revision => {
    const deadline = performance.now() + 35000;
    let forked = false;
    while (current(revision) && performance.now() < deadline) {
      let response;
      try {response = await request(`${root}/claims`, "POST", candidate ? {id: candidate} : {}, undefined, 4096, deadline);}
      catch (error) {if (current(revision) && performance.now() >= deadline) exhausted = true; throw error;}
      if (!current(revision)) return false;
      if (response.status === 201) {
        const body = response.body;
        if (!exactKeys(body, ["id", "claim", "expiresIn"]) || !validID(body.id) || !validID(body.claim) || body.claim.length < 20 ||
            body.expiresIn !== 30 || candidate && body.id !== candidate) {
          diagnose("claim", "response", response.requestID); return false;
        }
        candidate = body.id;
        lease = {id: body.id, claim: body.claim, ttl: body.expiresIn * 1000, until: performance.now() + body.expiresIn * 1000};
        try {sessionStorage.setItem(storageKey, candidate);} catch (_) {storageAvailable = false; diagnose("storage", "unavailable", "", "debug");}
        reconnect = true;
        return true;
      }
      if (response.status !== 409) {diagnose("claim", [401, 403].includes(response.status) ? "ownership" : "server", response.requestID); return false;}
      diagnose("claim", "occupied", response.requestID, "debug");
      status("waiting");
      if (!reconnect && !forked && candidate) {candidate = ""; forked = true; forkedCandidate = true; continue;}
      const seconds = Number(response.retry);
      if (!Number.isInteger(seconds) || seconds < 1 || seconds > 30) {diagnose("claim", "response", response.requestID); return false;}
      await wait(Math.min(seconds * 1000, Math.max(0, deadline - performance.now())));
    }
    if (current(revision)) {exhausted = true; diagnose("claim", "timeout");}
    return false;
  };
  const command = body => {
    if (!exactKeys(body, ["command", "position", "volume", "muted", "itemId"])) return undefined;
    if (body.command === null) return Object.keys(body).length === 1 ? null : undefined;
    if (!["play", "pause", "stop", "seek", "volume", "mute", "play_media"].includes(body.command)) return undefined;
    if (body.position !== undefined && (!Number.isFinite(body.position) || body.position < 0 || body.position > 1e9) ||
        body.volume !== undefined && (!Number.isFinite(body.volume) || body.volume < 0 || body.volume > 1) ||
        body.muted !== undefined && typeof body.muted !== "boolean" ||
        body.itemId !== undefined && (typeof body.itemId !== "string" || !body.itemId || body.itemId.length > 128 || /[\u0000-\u001f\u007f]/.test(body.itemId))) return undefined;
    if (body.command === "play_media" && !body.itemId) return undefined;
    return {...body, position: body.position ?? 0, volume: body.volume ?? 0, muted: body.muted ?? false};
  };
  const publish = async revision => {
    let operation = "profile";
    try {
      if (!await viewer(revision)) return false;
      if (!lease || lease.until <= performance.now()) {
        lease = undefined; operation = "claim";
        status("connecting");
        if (!await acquire(revision)) return false;
      }
      if (!current(revision)) return false;
      const observed = adapter.snapshot(), authority = lease;
      operation = "state";
      const response = await request(`${root}/${authority.id}`, "PUT", observed.body, authority.claim, 4096);
      if (!current(revision) || !sameSource(observed.source) || lease !== authority) return false;
      if (response.status !== 200) {
        if ([401, 403].includes(response.status)) lease = undefined;
        diagnose(operation, [401, 403].includes(response.status) ? "ownership" : "server", response.requestID);
        return false;
      }
      const next = command(response.body);
      if (next === undefined) {diagnose(operation, "response", response.requestID); return false;}
      authority.until = performance.now() + authority.ttl;
      status("connected");
      if (next && await viewer(revision) && current(revision) && sameSource(observed.source) && lease === authority) {
        operation = "command";
        await adapter.apply(next);
      }
      return current(revision);
    } catch (error) {
      if (current(revision)) diagnose(operation, error.failure || "network", error.requestID);
      return false;
    }
  };
  const sync = () => {
    if (!owner()) {if (!terminal && !hidden) {stop(); terminal = true;} return Promise.resolve();}
    if (exhausted) return Promise.resolve();
    if (flight) {again = true; return flight;}
    const revision = generation;
    let task;
    task = (async () => {
      for (let turn = 0; turn < 2 && current(revision); turn++) {
        again = false;
        if (!await publish(revision)) {if (current(revision)) status("unavailable", true); break;}
        if (!again) break;
      }
    })().finally(() => {if (flight === task) flight = undefined;});
    flight = task;
    return task;
  };
  const stop = () => {
    generation++; again = false; flight = undefined;
    clearInterval(interval); interval = undefined;
    events?.close(); events = undefined;
    clearTimeout(retryTimer); retryTimer = undefined; retryResolve?.(); retryResolve = undefined;
    for (const controller of controllers) controller.abort();
    controllers.clear();
    const retired = lease; lease = undefined;
    status("stopped");
    if (retired) void fetch(`${root}/${retired.id}/release`, {method: "POST", credentials: "same-origin",
      headers: headers(retired.claim), body: "{}", keepalive: true}).catch(() => {});
  };
  const start = () => {
    if (!owner()) return;
    generation++; exhausted = false;
    if (!storageAvailable) diagnose("storage", "unavailable", "", "debug");
    interval = setInterval(sync, 5000);
    if (typeof EventSource !== "undefined") {
      events = new EventSource("/api/v1/events");
      events.addEventListener("home-assistant.command", event => {
        if (!lease || typeof event.data !== "string" || event.data.length > 1024) return;
        try {if (JSON.parse(event.data).resource === `${root}/${lease.id}`) void sync();} catch (_) {}
      });
    }
    void sync();
  };
  const hide = () => {hidden = true; stop();};
  const show = event => {if (!terminal && hidden && event.persisted) {hidden = false; reconnect = true; start();}};
  addEventListener("pagehide", hide);
  addEventListener("pageshow", show);
  start();
  return {sync, retry: () => {if (!owner() || flight) return flight || Promise.resolve(); exhausted = false; return sync();},
    close: () => {terminal = true; stop(); removeEventListener("pagehide", hide); removeEventListener("pageshow", show);}};
};
