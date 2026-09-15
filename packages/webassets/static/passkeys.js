const status = document.querySelector("[data-passkey-status]");
const messages = document.body.dataset;
const storageKey = messages.passkeyStorage || "kinosail-passkey";
const loginNext = document.querySelector("[data-login-next]")?.dataset.loginNext || messages.loginNext || new URLSearchParams(location.search).get("next") || "/";
let conditionalLogin;
let loginPending;
let manualLogin = false;

const message = (name, fallback) => messages[name] || fallback;

async function requestHeaders(kind) {
  const csrf = document.querySelector('meta[name="kinosail-csrf"]')?.content;
  if (csrf) return {"X-Kinosail-CSRF": csrf};
  if (kind !== "register") return {};
  const response = await fetch("/api/v1/me", {credentials: "same-origin", headers: {Accept: "application/json"}});
  if (!response.ok) throw new Error("Sign in before you add a passkey.");
  const sessionCSRF = (await response.json()).csrf;
  if (!sessionCSRF) throw new Error("Sign in before you add a passkey.");
  return {"X-Kinosail-CSRF": sessionCSRF};
}

async function ceremony(kind, {conditional = false, signal} = {}) {
  if (!window.isSecureContext || !window.PublicKeyCredential) {
    if (status) status.textContent = message("passkeyInsecure", "Passkeys need a secure connection and a supported browser. Open Kinosail at its secure address and try again.");
    return;
  }
  if (!conditional && status) status.textContent = message("passkeyWaiting", "Waiting for your passkey…");
  const headers = await requestHeaders(kind);
  const begin = await fetch(`/api/v1/passkeys/${kind}/begin`, {method: "POST", credentials: "same-origin", headers, signal});
  if (begin.status === 421 && begin.headers.get("location")) {
    if (conditional) return;
    return location.replace(begin.headers.get("location"));
  }
  if (!begin.ok) throw new Error(await begin.text() || "Could not start the passkey request. Try again.");
  const options = await begin.json();
  const publicKey = kind === "register"
    ? PublicKeyCredential.parseCreationOptionsFromJSON(options.publicKey)
    : PublicKeyCredential.parseRequestOptionsFromJSON(options.publicKey);
  const credential = kind === "register"
    ? await navigator.credentials.create({publicKey, signal})
    : await navigator.credentials.get({publicKey, signal, ...(conditional ? {mediation: "conditional"} : {})});
  if (!conditional && status) status.textContent = message("passkeyVerifying", "Verifying your passkey…");
  const finish = await fetch(`/api/v1/passkeys/${kind}/finish`, {
    method: "POST",
    credentials: "same-origin",
    headers: {"Content-Type": "application/json", ...(kind === "login" ? {"X-Kinosail-Login-Next": loginNext} : {}), ...headers},
    body: JSON.stringify(credential),
    signal,
  });
  if (!finish.ok) throw new Error(await finish.text() || "Could not verify your passkey. Try again.");
  if (kind === "login" && status) status.textContent = message("passkeySignedIn", "Signed in. Opening your library…");
  try { localStorage.setItem(storageKey, "1"); } catch { /* Passkey use must not depend on browser storage. */ }
  const query = new URLSearchParams(location.search);
  if (query.get("setup") === "1") return location.replace("/onboarding/connection");
  if (query.get("mfa") === "required") return location.replace("/");
  if (query.get("passkey") === "offer") return location.replace(loginNext);
  if (kind === "login") return location.replace(finish.headers.get("X-Kinosail-Login-Next") || loginNext);
  if (status) status.textContent = message("passkeyAdded", "Passkey added.");
}

function failed(error) {
  if (error.name === "AbortError") return;
  if (status) status.textContent = error.name === "SecurityError"
    ? message("passkeyInsecure", "Passkeys need a secure connection and a supported browser. Open Kinosail at its secure address and try again.")
    : error.name === "NotAllowedError" ? "The passkey request was not completed. Try again when you are ready." : message("passkeyFailed", "Could not use your passkey. Try again or use another sign-in method.");
}

function login(options) {
  if (loginPending) return loginPending;
  const pending = ceremony("login", options);
  loginPending = pending;
  pending.finally(() => { if (loginPending === pending) loginPending = null; }).catch(() => {});
  return pending;
}

document.querySelector("[data-passkey-add]")?.addEventListener("click", () => ceremony("register").catch(failed));
document.querySelector("[data-passkey-login]")?.addEventListener("click", () => {
  manualLogin = true;
  if (!conditionalLogin) return login().catch(failed);
  const pending = loginPending;
  conditionalLogin?.abort();
  conditionalLogin = null;
  Promise.resolve(pending).catch(() => {}).then(() => login().catch(failed));
});

async function offerConditionalLogin() {
  try {
    if (!document.querySelector('[autocomplete~="webauthn"]') ||
        !window.PublicKeyCredential?.isConditionalMediationAvailable ||
        !await PublicKeyCredential.isConditionalMediationAvailable() || manualLogin) return;
    conditionalLogin = new AbortController();
    await login({conditional: true, signal: conditionalLogin.signal});
  } catch (error) {
    if (error.name !== "AbortError" && error.name !== "NotAllowedError") failed(error);
  }
}

async function offerLogin() {
  let returning = false;
  try { returning = localStorage.getItem(storageKey) === "1"; } catch { /* Conditional autofill remains available below. */ }
  if (returning && document.querySelector("[data-passkey-login]")) {
    try {
      await login();
      return;
    } catch (error) {
      if (error.name === "AbortError") return;
      if (status) status.textContent = "";
    }
  }
  await offerConditionalLogin();
}

offerLogin();
