let csrf = "";

export function clearPrivateCache() {
  for (const key of ["board", "catalog", "recents", "usage"]) {
    try { localStorage.removeItem(`kinosail-dashboard-${key}-v1`); } catch {}
  }
}

export function setCSRF(value) { csrf = value || ""; }

export async function api(path, options = {}) {
  const request = { credentials: "same-origin", headers: { Accept: "application/json", ...(options.headers || {}) }, ...options };
  if (request.body && typeof request.body !== "string") {
    request.headers["Content-Type"] = "application/json";
    request.body = JSON.stringify(request.body);
  }
  if (request.method && request.method !== "GET" && request.method !== "HEAD" && csrf) request.headers["X-Kinosail-CSRF"] = csrf;
  let response;
  try { response = await fetch(path, request); }
  catch { throw Object.assign(new Error("Could not reach the Server."), {offline: true}); }
  if (path === "/api/v1/session" && request.method === "DELETE" && response.ok) clearPrivateCache();
  if (response.status === 401) {
    clearPrivateCache();
    window.dispatchEvent(new Event("kinosail:session-ended"));
    location.assign("/login");
    throw new Error("Your session ended. Sign in again.");
  }
  if (response.status === 204) return null;
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error || `Request failed with status ${response.status}`);
  return data;
}

export const get = path => api(path);
export const post = (path, body = {}) => api(path, { method: "POST", body });
export const put = (path, body) => api(path, { method: "PUT", body });
export const patch = (path, body) => api(path, { method: "PATCH", body });
export const remove = (path, body) => api(path, { method: "DELETE", body });
