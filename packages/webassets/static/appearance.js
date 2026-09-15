const themeKey = "kinosail-theme";
const themes = ["dark", "light", "system"];
const systemTheme = matchMedia("(prefers-color-scheme: dark)");
let theme;
try { theme = localStorage.getItem(themeKey) || "dark"; } catch { theme = "dark"; }
if (!themes.includes(theme)) theme = "dark";

function applyTheme() {
  const effective = theme === "system" ? systemTheme.matches ? "dark" : "light" : theme;
  document.documentElement.dataset.theme = effective;
  document.querySelector('meta[name="theme-color"]')?.setAttribute("content", effective === "light" ? "#f4f8ef" : "#0b0d0b");
  document.querySelectorAll("[data-theme-choice]").forEach((control) => {
    if (control instanceof HTMLInputElement && control.type === "radio") control.checked = control.value === theme;
    else control.value = theme;
  });
}

function bindThemeControls() {
  document.querySelectorAll("[data-theme-choice]").forEach((control) => control.addEventListener("change", () => {
    if (control instanceof HTMLInputElement && control.type === "radio" && !control.checked) return;
    if (!themes.includes(control.value)) return;
    theme = control.value;
    try { localStorage.setItem(themeKey, theme); } catch {}
    applyTheme();
  }));
  applyTheme();
}


applyTheme();
if (document.readyState === "loading") addEventListener("DOMContentLoaded", bindThemeControls, { once: true });
else bindThemeControls();
systemTheme.addEventListener?.("change", () => { if (theme === "system") applyTheme(); });
