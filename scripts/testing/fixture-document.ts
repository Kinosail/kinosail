// Render exported application fixtures without loading external scripts.
export function fixtureDocument({ source, css, dark = false }: { source: string; css: string; dark?: boolean }): string {
  const document = new DOMParser().parseFromString(source, "text/html");
  if (dark) document.documentElement.dataset.theme = "dark";
  const stylesheet = document.querySelector('link[href*="app.css"]');
  if (stylesheet) {
    const style = document.createElement("style");
    style.textContent = css;
    stylesheet.replaceWith(style);
  }
  for (const script of document.querySelectorAll("script[src]")) script.remove();
  return `<!doctype html>${document.documentElement.outerHTML}`;
}
