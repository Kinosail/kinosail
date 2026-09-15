const iconNames = new Set(["app", "search", "settings", "refresh", "edit", "plus", "close", "grid", "external", "monitor", "grip", "up", "down", "star", "play", "image", "headphones", "tv", "film", "music", "captions", "download", "home", "shield", "server", "network", "database", "chart", "pulse", "boxes", "file", "cloud", "lock"]);

export function icon(name, label = "") {
  const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  const use = document.createElementNS("http://www.w3.org/2000/svg", "use");
  use.setAttribute("href", `/static/icons.svg#${iconNames.has(name) ? name : "app"}`);
  svg.append(use);
  if (label) svg.setAttribute("aria-label", label);
  else svg.setAttribute("aria-hidden", "true");
  return svg;
}

export function brandMark(name, logo = "app", accent = "slate") {
  const mark = element("span", "app-mark");
  mark.dataset.logo = logo || "app";
  mark.dataset.accent = accent || "slate";
  mark.setAttribute("aria-hidden", "true");
  const words = String(name || "App").trim().split(/\s+/u).filter(Boolean);
  const monogram = words.length > 1 ? words.slice(0, 2).map(word => word[0]).join("") : (words[0] || "A").slice(0, 2);
  mark.append(element("span", "app-monogram", monogram.toLocaleUpperCase()));
  return mark;
}

export function element(tag, className = "", text = "") {
  const node = document.createElement(tag);
  if (className) node.className = className;
  if (text) node.textContent = text;
  return node;
}

export function button(className, label, iconName = "") {
  const node = element("button", className);
  node.type = "button";
  if (iconName) node.append(icon(iconName));
  node.append(document.createTextNode(label));
  return node;
}

export function clear(node) { node.replaceChildren(); }

export function selectedChoice(id) { return field(id).querySelector("input:checked")?.value || ""; }

export function selectChoice(id, value) {
  const input = [...field(id).querySelectorAll("input")].find(option => option.value === value);
  if (input) input.checked = true;
}

export function formatAge(value) {
  if (!value) return "not checked";
  const seconds = Math.max(0, Math.round((Date.now() - new Date(value).getTime()) / 1000));
  if (seconds < 10) return "just now";
  if (seconds < 60) return `${seconds}s ago`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 60) return `${minutes}m ago`;
  return `${Math.round(minutes / 60)}h ago`;
}

export function healthText(health) {
  const names = { reachable: "Reachable", slow: "Slow response", degraded: "Needs attention", unavailable: "Unavailable", checking: "Checking", unchecked: "Not checked", disabled: "Checks off" };
  return names[health?.state] || "Not checked";
}

export function field(id) { return document.getElementById(id); }
