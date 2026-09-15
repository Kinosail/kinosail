let csrf = "";

export function setCSRF(value) { csrf = value || ""; }

export async function api(path, options = {}) {
  const request = { credentials: "same-origin", headers: { Accept: "application/json", ...(options.headers || {}) }, ...options };
  if (request.body && typeof request.body !== "string") {
    request.headers["Content-Type"] = "application/json";
    request.body = JSON.stringify(request.body);
  }
  if (request.method && request.method !== "GET" && request.method !== "HEAD" && csrf) request.headers["X-Kinosail-CSRF"] = csrf;
  const response = await fetch(path, request);
  if (response.status === 401) {
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
