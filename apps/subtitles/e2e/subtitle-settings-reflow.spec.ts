import {writeFile} from "node:fs/promises";
import AxeBuilder from "@axe-core/playwright";
import {expect, test} from "@playwright/test";
import {login} from "./test-instance-helpers";

test.skip(process.env.KINOSAIL_TEST_INSTANCE !== "1", "requires the populated public test instance");

for (const viewport of [{width:320,height:800},{width:390,height:844},{width:844,height:390}]) {
  for (const scale of ["100%","200%"]) test(`Settings controls reflow at ${viewport.width} with ${scale} text`,async({page},info)=>{
    await page.setViewportSize(viewport);
    await login(page);
    await page.addInitScript(scale=>{
      const apply=()=>{if(!document.documentElement)return false;document.documentElement.style.fontSize=scale;return true;};
      if(!apply()) {
        const observer=new MutationObserver(()=>{if(apply())observer.disconnect();});
        observer.observe(document,{childList:true});
      }
    },scale);
    for (const category of ["provider","security"]) {
      await page.goto(`/settings#${category}`,{waitUntil:"domcontentloaded"});
      await expect(page.locator(".settings-shell")).toBeVisible();
      await expect(page.locator("#session-timeouts")).toBeVisible();
      // Focus in the navigation retains the normal motion policy.
      await page.locator(".app-header a").first().focus();
      await expect(page.locator("html")).toHaveCSS("scroll-behavior","smooth");
      const geometry=await page.evaluate(()=>({
        overflow:document.documentElement.scrollWidth-innerWidth,scrollX,scrollY,
        controls:[...document.querySelectorAll<HTMLSelectElement>("#session-timeouts select")].map(node=>({
          value:node.value,width:node.getBoundingClientRect().width,parentWidth:node.parentElement!.getBoundingClientRect().width,
        })),
        containers:[...document.querySelectorAll("body *:not(option):not(optgroup)")]
          .filter(node=>node.clientWidth>0&&node.scrollWidth>node.clientWidth+1).slice(0,20)
          .map(node=>({tag:node.tagName,id:node.id,className:node.className,clientWidth:node.clientWidth,scrollWidth:node.scrollWidth,overflowX:getComputedStyle(node).overflowX})),
      }));
      await writeFile(info.outputPath(`geometry-${category}.json`),JSON.stringify({viewport,scale,...geometry}));
      expect(geometry.overflow).toBeLessThanOrEqual(1);
      const languages=page.locator(".subtitle-language-list");
      const rows=await languages.locator("li").evaluateAll(nodes=>nodes.map(node=>({
        width:node.getBoundingClientRect().width,parent:node.parentElement!.getBoundingClientRect().width,
        details:node.querySelector("span")!.getBoundingClientRect().width,
        buttons:[...node.querySelectorAll("button")].map(button=>({width:button.getBoundingClientRect().width,height:button.getBoundingClientRect().height})),
      })));
      expect(rows.length).toBeGreaterThan(0);
      for(const row of rows) {
        expect(row.width).toBeLessThanOrEqual(row.parent+1);
        if(viewport.width<=390) expect(row.details).toBeGreaterThanOrEqual(Math.min(row.parent,150)-1);
        for(const button of row.buttons) {expect(button.width).toBeGreaterThanOrEqual(44);expect(button.height).toBeGreaterThanOrEqual(44);}
      }
      await languages.scrollIntoViewIfNeeded();
      await page.screenshot({path:info.outputPath(`languages-${category}-${viewport.width}-${scale}.png`)});
      expect((await new AxeBuilder({page}).include(".subtitle-language-list").analyze()).violations).toEqual([]);
      expect(geometry.controls).toHaveLength(4);
      for(const control of geometry.controls) {
        expect(control.width).toBeGreaterThan(44);
        expect(control.width).toBeLessThanOrEqual(control.parentWidth+1);
      }
      expect((await new AxeBuilder({page}).include("#session-timeouts").analyze()).violations).toEqual([]);
      const last=page.getByRole("button",{name:"Save public timeouts",exact:true});
      await last.scrollIntoViewIfNeeded();
      await last.focus();
      await page.keyboard.press("Shift+Tab");
      await page.keyboard.press(info.project.name === "webkit" ? "Alt+Tab" : "Tab");
      await expect(last).toBeFocused();
      await expect(page.locator("html")).toHaveCSS("scroll-behavior","auto");
      await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
      const focus=await last.evaluate(node=>{
        const rect=node.getBoundingClientRect(),dock=document.querySelector("[data-subtitle-dock]")?.getBoundingClientRect(),header=document.querySelector(".app-header")?.getBoundingClientRect();
        return {rect:rect.toJSON(),dock:dock?.toJSON(),header:header?.toJSON(),viewport:{width:innerWidth,height:innerHeight},
          exposed:rect.left>=0&&rect.right<=innerWidth+1&&rect.top>=(header?.bottom??0)-1&&rect.bottom<=(dock?.top??innerHeight)+1};
      });
      await writeFile(info.outputPath(`focus-${category}.json`),JSON.stringify(focus));
      expect(focus.exposed).toBe(true);
      await page.screenshot({path:info.outputPath(`settings-${category}-${viewport.width}-${scale}.png`)});
    }
  });
}
