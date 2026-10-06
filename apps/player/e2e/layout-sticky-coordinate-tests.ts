import {expect, test} from "@playwright/test";
import {subtitleSearchGeometry,subtitleSearchUnchanged} from "../../../scripts/testing/layout-stability-subtitle-search.mjs";

test("Natural scrolling preserves a not-yet-stuck subtitle filter while genuine geometry changes fail",async({page})=>{
  await page.setViewportSize({width:320,height:800});
  await page.setContent(`<style>body{margin:0}.app-header{position:fixed;top:0;width:100%;height:40px}.spacer{height:1200px}.subtitle-filters{position:sticky;top:40px;height:50px}#subtitle-content{height:1600px}</style><header class="app-header"></header><div class="spacer"></div><h2 id="subtitle-list-title">Library</h2><form class="subtitle-filters"><input id="search"></form><section id="subtitle-content">Content</section>`);
  const before=await subtitleSearchGeometry(page);
  await page.locator("#search").scrollIntoViewIfNeeded();
  const after=await subtitleSearchGeometry(page);
  expect(Math.abs(before[2].y-after[2].y)).toBeGreaterThan(1);
  expect(Math.abs(before[2].documentY-after[2].documentY)).toBeLessThanOrEqual(1);
  expect(subtitleSearchUnchanged(before,after)).toBe(true);
  await page.locator("#subtitle-content").evaluate(node=>(node as HTMLElement).style.height="1601.5px");
  expect(subtitleSearchUnchanged(after,await subtitleSearchGeometry(page))).toBe(false);
});

test("A stuck subtitle filter retains viewport pinning and detects a changed sticky inset",async({page})=>{
  await page.setContent(`<style>body{margin:0}.app-header{position:fixed;top:0;height:40px;width:100%}.spacer{height:1200px}.subtitle-filters{position:sticky;top:40px;height:50px}#subtitle-content{height:1600px}</style><header class="app-header"></header><div class="spacer"></div><h2 id="subtitle-list-title">Library</h2><form class="subtitle-filters"><input></form><section id="subtitle-content">Content</section>`);
  await page.evaluate(()=>scrollTo(0,1400));
  const before=await subtitleSearchGeometry(page);
  await page.evaluate(()=>scrollTo(0,1450));
  expect(subtitleSearchUnchanged(before,await subtitleSearchGeometry(page))).toBe(true);
  await page.locator(".subtitle-filters").evaluate(node=>(node as HTMLElement).style.top="44px");
  expect(subtitleSearchUnchanged(before,await subtitleSearchGeometry(page))).toBe(false);
});

test("A sticky filter above its inset at the containing-block edge follows document coordinates",async({page})=>{
  await page.setContent(`<style>body{margin:0}.app-header{position:fixed;top:0;height:40px;width:100%}.spacer{height:600px}.container{height:200px}.subtitle-filters{position:sticky;top:80px;height:50px}#subtitle-content{height:1600px}</style><header class="app-header"></header><div class="spacer"></div><h2 id="subtitle-list-title">Library</h2><div class="container"><form class="subtitle-filters"><input></form></div><section id="subtitle-content">Content</section>`);
  await page.evaluate(()=>scrollTo(0,800));
  const before=await subtitleSearchGeometry(page);
  expect(before[2].y).toBeLessThan(79);
  expect(before[2].pinned).toBe(false);
  await page.evaluate(()=>scrollBy(0,10));
  expect(subtitleSearchUnchanged(before,await subtitleSearchGeometry(page))).toBe(true);
});
