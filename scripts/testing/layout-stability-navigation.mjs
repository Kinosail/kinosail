// Measure rendered text in the real Player page without changing its layout.
export function personalNavigationSnapshot() {
  const nav = document.querySelector("[data-mobile-tabs]");
  if (!nav) return {present: false, entries: []};
  const visible = node => {
    const box = node.getBoundingClientRect();
    return box.width > 0 && box.height > 0 && (!node.checkVisibility || node.checkVisibility());
  };
  const entries = [...nav.querySelectorAll(":scope > a, :scope > .nav-more > summary")]
    .filter(visible).map(link => {
      const box = link.getBoundingClientRect(), words = [];
      const walker = document.createTreeWalker(link, NodeFilter.SHOW_TEXT);
      for (let node = walker.nextNode(); node; node = walker.nextNode()) {
        for (const match of node.textContent.matchAll(/\S+/g)) {
          const range = document.createRange();
          range.setStart(node, match.index);
          range.setEnd(node, match.index + match[0].length);
          const rects = [...range.getClientRects()].filter(rect => rect.width > 0 && rect.height > 0);
          const lines = new Set(rects.map(rect => Math.round(rect.top))).size;
          const contained = rects.length > 0 && rects.every(rect => rect.left >= box.left - 1 &&
            rect.right <= box.right + 1 && rect.top >= box.top - 1 && rect.bottom <= box.bottom + 1);
          words.push({text: match[0], lines, contained});
        }
      }
      return {label: link.textContent.trim(), words, box: box.toJSON()};
    });
  return {present: visible(nav), ready: nav.classList.contains("has-personal-tabs"), entries};
}

export function personalNavigationReadable(initial, settled, expected) {
  return settled.present && settled.ready &&
    JSON.stringify(settled.entries.map(entry => entry.label)) === JSON.stringify(expected) &&
    [initial, settled].every(state => state.entries.every(entry => entry.words.length > 0 &&
      entry.words.every(word => word.lines === 1 && word.contained)));
}

export async function measurePersonalNavigationFlow(browser, options, results, probe) {
  for (const viewport of [{width: 320, height: 800}, {width: 390, height: 844}, {width: 844, height: 390}]) {
    for (const savedTabs of [false, true]) {
      probe.stage = "player-enlarged-dock-end-focus";
      const context = await browser.newContext({...options, viewport, ignoreHTTPSErrors: false, reducedMotion: "reduce",
        userAgent: "Mozilla/5.0 (iPhone; CPU iPhone OS 18_0 like Mac OS X) AppleWebKit/605.1.15 Version/18.0 Mobile/15E148 Safari/604.1"});
      let page;
      try {
        await context.addInitScript(() => {
          const apply = () => {
            if (!document.documentElement) return false;
            document.documentElement.style.fontSize = "200%";
            return true;
          };
          if (!apply()) {
            const observer = new MutationObserver(() => { if (apply()) observer.disconnect(); });
            observer.observe(document, {childList: true});
          }
        });
        page = await probe.openPage(context);
        await page.goto("/settings#thanks");
        const nav = page.locator("[data-mobile-tabs]");
        if (savedTabs) {
          const profile = await nav.getAttribute("data-nav-profile");
          if (!profile) throw new Error("Player navigation profile missing");
          await page.evaluate(profile => localStorage.setItem(`kinosail:tabs:v2:${profile}`,
            '["audiobooks","collections","unwatched","playlists"]'), profile);
          await page.reload();
        }
        await page.waitForFunction(() => document.querySelector("[data-mobile-tabs]")?.classList.contains("has-personal-tabs"));
        const navigation = await page.evaluate(personalNavigationSnapshot);
        const expected = savedTabs ? ["Audiobooks", "Collections", "Unwatched", "Playlists", "More"] :
          ["Home", "TV Shows", "Movies", "Search", "More"];
        if (!navigation.present || !navigation.ready ||
            JSON.stringify(navigation.entries.map(entry => entry.label)) !== JSON.stringify(expected)) {
          results.push({flow: probe.stage, viewport, savedTabs, navigation, expected, stable: false});
          throw new Error("Player navigation did not render the requested tabs");
        }
        const last = page.locator("#thanks a[href]").last();
        await last.focus();
        await page.keyboard.press("Shift+Tab");
        await page.keyboard.press("Tab");
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(resolve)));
        const control = await last.boundingBox(), dock = await nav.boundingBox();
        const header = await page.locator(".app-header").boundingBox();
        const focusRetained = await last.evaluate(node => node === document.activeElement);
        await page.evaluate(() => scrollTo({top: document.documentElement.scrollHeight, behavior: "instant"}));
        const tail = await page.locator("#thanks").boundingBox(), endDock = await nav.boundingBox();
        const endFocus = {flow: "player-enlarged-dock-end-focus", viewport, savedTabs, navigation, control, dock, header, tail, endDock,
          focusRetained, stable: Boolean(control && dock && header && tail && endDock &&
            control.y >= header.y + header.height - 1 && control.y + control.height <= dock.y + 1 &&
            tail.y + tail.height <= endDock.y + 1)};
        results.push(endFocus);
        if (!focusRetained || !endFocus.stable) throw new Error("Player page end is obscured by its navigation");

        probe.stage = "player-enlarged-more-keyboard";
        const more = nav.locator(":scope > .nav-more > summary");
        await more.focus();
        await page.keyboard.press("Enter");
        const menu = nav.locator(".nav-more-menu");
        await menu.waitFor({state: "visible"});
        const connect = menu.getByRole("link", {name: "Quick Connect", exact: true});
        await connect.focus();
        const measureMenuControl = async locator => {
          const control = await locator.boundingBox(), menuBox = await menu.boundingBox(), dock = await nav.boundingBox();
          return {control, menuBox, dock, reachable: Boolean(control && menuBox && dock &&
            menuBox.y >= 0 && menuBox.x >= 0 && menuBox.x + menuBox.width <= viewport.width + 1 &&
            menuBox.y + menuBox.height <= Math.min(viewport.height, dock.y) + 1 &&
            control.x >= menuBox.x - 1 && control.x + control.width <= menuBox.x + menuBox.width + 1 &&
            control.y >= menuBox.y - 1 && control.y + control.height <= menuBox.y + menuBox.height + 1)};
        };
        const connectGeometry = await measureMenuControl(connect);
        const customize = menu.getByRole("button", {name: "Customize tabs", exact: true});
        await customize.focus();
        const customizeGeometry = await measureMenuControl(customize);
        const menuReachable = connectGeometry.reachable && customizeGeometry.reachable;
        const menuResult = {flow: "player-enlarged-more-keyboard", viewport, savedTabs, navigation,
          connectGeometry, customizeGeometry, menuReachable, stable: menuReachable};
        results.push(menuResult);
        if (!menuReachable) throw new Error("Player More controls are obscured by its navigation");
        await page.keyboard.press("Enter");
        const editor = page.locator(".tab-editor");
        await editor.waitFor({state: "visible"});
        await page.keyboard.press("Escape");
        await editor.waitFor({state: "hidden"});
        const focusRestored = await more.evaluate(node => node === document.activeElement);
        menuResult.focusRetained = focusRestored;
        if (!focusRestored) throw new Error("Player Customize tabs did not restore focus to More");
        await page.keyboard.press("Enter");
        await connect.click();
        await page.waitForURL(url => url.pathname === "/quick-connect");
        await page.getByRole("heading", {name: "Connect a device", exact: true}).waitFor({state: "visible"});
        const overflow = await page.evaluate(() => document.documentElement.scrollWidth - innerWidth);
        Object.assign(menuResult, {destinationReached: true, overflow});
        if (overflow > 1) throw new Error("Player Quick Connect exceeds the enlarged mobile viewport");

        probe.stage = "player-enlarged-library-overlays";
        await page.goto("/?view=movies");
        const jump = page.locator("[data-title-jump]");
        await page.waitForFunction(() => document.querySelector("[data-title-jump]")?.classList.contains("title-jump-scrubbable"));
        const index = jump.locator("[data-title-jump-index]");
        const railVisible = await index.isVisible();
        const railFixed = await index.evaluate(node => getComputedStyle(node).position === "fixed");
        if (railVisible && !railFixed) {
          await index.locator("[data-title-letter]").first().focus();
          await page.keyboard.press("Shift+Tab");
          await page.keyboard.press("Tab");
          await page.evaluate(() => new Promise(resolve => requestAnimationFrame(resolve)));
        }
        const rail = await index.boundingBox(), libraryDock = await nav.boundingBox();
        const libraryHeader = await page.locator(".app-header").boundingBox();
        let railReachable, letterControls = [], indexFocusRetained;
        if (railVisible) {
          letterControls = await index.locator("[data-title-letter]").evaluateAll(nodes => nodes.map(node => {
            const style = getComputedStyle(node), focused = node === document.activeElement;
            const outline = focused && node.matches(":focus-visible") && style.outlineStyle !== "none" ?
              Math.max(0, (parseFloat(style.outlineWidth) || 0) + (parseFloat(style.outlineOffset) || 0)) : 0;
            return {rect: node.getBoundingClientRect().toJSON(), focused, outline};
          }));
          indexFocusRetained = railFixed || letterControls[0]?.focused;
          railReachable = Boolean(rail && libraryDock && libraryHeader && letterControls.length && indexFocusRetained &&
            (!railFixed || (rail.y >= libraryHeader.y + libraryHeader.height - 1 && rail.y + rail.height <= libraryDock.y + 1)) &&
            letterControls.every(({rect: r, outline}) => r.width > 0 && r.height > 0 &&
              r.top >= rail.y - 1 && r.bottom <= rail.y + rail.height + 1 &&
              r.left >= rail.x - 1 && r.right <= rail.x + rail.width + 1 &&
              r.top - outline >= libraryHeader.y + libraryHeader.height - 1 && r.bottom + outline <= libraryDock.y + 1));
        } else {
          await jump.locator("[data-title-jump-open]").click();
          const dialog = jump.locator("[data-title-jump-dialog]");
          await dialog.waitFor({state: "visible"});
          await dialog.locator("[data-title-letter]").first().click();
          await dialog.waitFor({state: "hidden"});
          railReachable = true;
        }
        const overlayResult = {flow: probe.stage, viewport, savedTabs, railVisible, railFixed, rail, letterControls, indexFocusRetained,
          libraryDock, libraryHeader, railReachable,
          userAgentFixture: "iPhone web install affordance; browser engine unchanged", stable: railReachable};
        results.push(overlayResult);
        if (!railReachable) throw new Error("Player title index is obscured by its navigation");
        await more.click();
        await menu.locator("[data-install]").click();
        const help = page.locator("[data-install-help]");
        await help.waitFor({state: "visible"});
        const closeHelp = help.locator("[data-install-close]");
        await closeHelp.focus();
        const helpBox = await help.boundingBox(), closeBox = await closeHelp.boundingBox(), helpDock = await nav.boundingBox();
        const helpFocusSamples = await help.evaluate(async panel => {
          const button = panel.querySelector("[data-install-close]"), samples = [];
          for (const frameCount of [0, 2, 4]) {
            for (let frame = 0; frame < frameCount; frame++) await new Promise(resolve => requestAnimationFrame(resolve));
            const style = getComputedStyle(panel);
            samples.push({time: performance.now(), focused: document.activeElement === button,
              focusVisible: button.matches(":focus-visible"), scrollTop: panel.scrollTop,
              clientHeight: panel.clientHeight, scrollHeight: panel.scrollHeight,
              overflowY: style.overflowY, scrollBehavior: style.scrollBehavior,
              panel: panel.getBoundingClientRect().toJSON(), button: button.getBoundingClientRect().toJSON()});
          }
          return samples;
        });
        const helpReachable = Boolean(helpBox && closeBox && helpDock && helpBox.y >= 0 &&
          helpBox.x >= 0 && helpBox.x + helpBox.width <= viewport.width + 1 &&
          helpBox.y + helpBox.height <= helpDock.y + 1 && closeBox.y >= helpBox.y - 1 &&
          closeBox.y + closeBox.height <= helpBox.y + helpBox.height + 1);
        Object.assign(overlayResult, {helpBox, closeBox, helpDock, helpFocusSamples, helpReachable, stable: railReachable && helpReachable});
        if (!helpReachable) throw new Error("Player install help is obscured by its navigation");
        await page.keyboard.press("Shift+Tab");
        await page.keyboard.press("Tab");
        const helpKeyboard = await closeHelp.evaluate(button => {
          const panel = button.closest("[data-install-help]"), style = getComputedStyle(button);
          return {focused: document.activeElement === button, focusVisible: button.matches(":focus-visible"),
            outline: Math.max(0, (parseFloat(style.outlineWidth) || 0) + (parseFloat(style.outlineOffset) || 0)),
            button: button.getBoundingClientRect().toJSON(), panel: panel.getBoundingClientRect().toJSON()};
        });
        const helpKeyboardReachable = helpKeyboard.focused && helpKeyboard.focusVisible &&
          helpKeyboard.button.top - helpKeyboard.outline >= helpKeyboard.panel.top &&
          helpKeyboard.button.bottom + helpKeyboard.outline <= helpKeyboard.panel.bottom;
        Object.assign(overlayResult, {helpKeyboard, helpKeyboardReachable, stable: overlayResult.stable && helpKeyboardReachable});
        if (!helpKeyboardReachable) throw new Error("Player install help keyboard focus is clipped");
        await probe.captureSnapshot(`${viewport.width}-${savedTabs ? "saved" : "default"}-install-help-keyboard`, page);
        await page.keyboard.press("Enter");
        await help.waitFor({state: "hidden"});
        overlayResult.helpDismissed = true;
      } catch (error) {
        await probe.captureFailure(error, page);
        throw error;
      } finally {
        await context.close();
      }
    }
  }
}
