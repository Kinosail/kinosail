(() => {
  const start = document.querySelector("[data-public-start]");
  if (!start) return;
  const cancel = document.querySelector("[data-public-cancel]");
  const pending = document.querySelector("[data-public-pending]");
  const code = document.querySelector("[data-public-code]");
  const status = document.querySelector("[data-public-status]");
  let timer, expires = 0, generation = 0;

  async function send(path) {
    const csrf = document.querySelector('meta[name="kinosail-csrf"]')?.content;
    return fetch(`/auth/quick-connect${path}`, {
      method: "POST", credentials: "same-origin", cache: "no-store",
      headers: csrf ? {"X-Kinosail-CSRF": csrf} : {},
      signal: AbortSignal.timeout(15000),
    });
  }

  function reset(message) {
    clearTimeout(timer);
    generation += 1;
    code.textContent = "";
    pending.hidden = true;
    start.disabled = false;
    cancel.disabled = false;
    status.textContent = message;
  }

  async function poll(current) {
    if (current !== generation) return;
    if (Date.now() >= expires) {
      reset("This code expired. Get a new sign-in code to try again.");
      return;
    }
    try {
      const response = await send("/token");
      if (current !== generation) return;
      if (response.status === 204) {
        status.textContent = "Signed in. Opening your library…";
        const next = new URL(document.body.dataset.loginNext || "/", location.origin);
        location.replace(next.origin === location.origin ? next.href : "/");
        return;
      }
      if (response.status === 202) {
        timer = setTimeout(() => poll(current), 3000);
        return;
      }
      reset(response.status === 429
        ? "Too many attempts. Wait a minute, then get a new code."
        : "This code is no longer available. Get a new code and ask your Viewer to approve it.");
    } catch {
      if (current !== generation) return;
      reset("The connection was interrupted. Check your connection, then get a new code.");
    }
  }

  start.disabled = false;
  start.addEventListener("click", async () => {
    start.disabled = true;
    status.textContent = "Getting your sign-in code…";
    const current = ++generation;
    try {
      const response = await send("");
      if (current !== generation) return;
      if (response.status !== 201) {
        reset(response.status === 429 ? "Too many attempts. Wait a minute and try again." : "Could not request a code. Refresh this page and try again.");
        return;
      }
      const body = await response.text();
      if (body.length > 1024) throw new Error("Invalid code response");
      const result = JSON.parse(body);
      if (!result || Object.keys(result).length !== 2 || typeof result.code !== "string" || !/^[0-9]{6}$/.test(result.code) || !Number.isInteger(result.expiresIn) || result.expiresIn <= 0 || result.expiresIn > 3600) throw new Error("Invalid code response");
      code.textContent = result.code;
      expires = Date.now() + result.expiresIn * 1000;
      pending.hidden = false;
      status.textContent = "Waiting for your Viewer to approve this browser…";
      document.querySelector("[data-public-code-label]").focus();
      timer = setTimeout(() => poll(current), 3000);
    } catch {
      if (current === generation) reset("Could not connect to the Server. Check your connection and try again.");
    }
  });

  cancel.addEventListener("click", async () => {
    clearTimeout(timer);
    generation += 1;
    cancel.disabled = true;
    try {
      const response = await send("/cancel");
      if (!response.ok) throw new Error("Cancellation failed");
      reset("Sign-in canceled.");
    } catch {
      reset("Could not confirm cancellation. Do not approve that code; it will expire shortly.");
    }
    start.focus();
  });
  window.addEventListener("pageshow", event => { if (event.persisted) reset("Get a new sign-in code to connect this browser."); });
  window.addEventListener("pagehide", () => { clearTimeout(timer); generation += 1; });
})();
