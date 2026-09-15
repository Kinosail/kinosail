const scopes = new WeakMap<Record<string, string>, string>();
const launch = `${Date.now()}-${Math.random().toString(36).slice(2)}`;
let sequence = 0;

// Cache identity belongs to the client session, never to a reusable protected URL.
export function artworkSource(uri: string, headers?: Record<string, string>) {
  if (!headers) return { uri };
  let scope = scopes.get(headers);
  if (!scope) {
    scope = `${launch}-${++sequence}`;
    scopes.set(headers, scope);
  }
  return { uri, headers, cacheKey: `${scope}:${uri}` };
}
