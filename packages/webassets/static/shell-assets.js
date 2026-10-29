const shell = {"/offline": "text/html", "/static/app.css": "text/css", "/static/manrope.woff2": "font/woff2", "/static/main.kinosail.bundle.js": "text/javascript", "/static/theme.js": "text/javascript", "/static/pwa.js": "text/javascript", "/static/icon.svg": "image/svg+xml", "/static/icon-192.png": "image/png", "/static/icon-512.png": "image/png", "/static/icon-maskable-512.png": "image/png", "/static/apple-touch-icon.png": "image/png", "/static/cinema-backdrop.jpg": "image/jpeg"};
const fetchShell = async (path, expectedType) => {
  const response = await fetch(path, {credentials: path === "/offline" ? "same-origin" : "omit", cache: "reload"});
  const responseURL = new URL(response.url);
  const responseType = response.headers.get("Content-Type")?.split(";", 1)[0];
  if (response.status !== 200 || response.redirected || responseURL.origin !== self.location.origin || responseURL.pathname !== path || responseType !== expectedType) throw new Error(`shell asset unavailable: ${path}`);
  return response;
};
