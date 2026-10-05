import "./subtitle-dock-focus-tests";
import "./layout-sticky-coordinate-tests";
import "./layout-response-lifecycle-tests";
import {readFile} from "node:fs/promises";
import {expect, test} from "@playwright/test";

const css = (await Promise.all([
  "../../../packages/webassets/static/player-app.css", "../../../packages/webassets/static/last-light.css",
  "../internal/server/static/home.css",
].map(path => readFile(new URL(path, import.meta.url), "utf8")))).join("\n");

for (const viewport of [{width: 320, height: 800}, {width: 390, height: 844}, {width: 844, height: 390}, {width: 1440, height: 900}]) {
  test(`pending controls and small targets preserve geometry at ${viewport.width}`, {tag: "@smoke"}, async ({page}, info) => {
    await page.setViewportSize(viewport);
    await page.route("**/*", route => route.fulfill({contentType: "text/html", body: `<html><body>
      <main><form><button id="save">Save settings</button><button id="cancel">Cancel</button></form>
      <section id="target"><p>Current content</p></section><p id="after">Following content remains in place.</p></main>
      </body></html>`}));
    await page.goto("/");
    await page.addStyleTag({content: css});
    const selectors = ["#save", "#cancel", "#target", "#after"];
    const before = await Promise.all(selectors.map(selector => page.locator(selector).boundingBox()));
    await page.locator("#save").evaluate(button => button.setAttribute("aria-busy", "true"));
    await page.locator("#target").evaluate(target => { target.classList.add("request-skeleton"); target.setAttribute("aria-busy", "true"); });
    const pending = await Promise.all(selectors.map(selector => page.locator(selector).boundingBox()));
    await info.attach("pending-geometry", {body: JSON.stringify({viewport, before, pending, data: "Synthetic markup with production CSS"}), contentType: "application/json"});
    for (let index = 0; index < selectors.length; index++) expect(pending[index], selectors[index]).toEqual(before[index]);
    await page.locator("#target").evaluate(target => { target.classList.remove("request-skeleton"); target.removeAttribute("aria-busy"); });
    await page.locator("#save").evaluate(button => button.removeAttribute("aria-busy"));
    await page.locator("#save").focus();
    await expect(page.locator("#save")).toBeFocused();
    // Safari's default macOS keyboard preference tabs to buttons with Option.
    await page.keyboard.press(info.project.name === "webkit" ? "Alt+Tab" : "Tab");
    await expect(page.locator("#cancel")).toBeFocused();
    await page.emulateMedia({reducedMotion: "reduce"});
    await expect(page.locator("#save")).toHaveCSS("animation-name", "none");
  });
}
