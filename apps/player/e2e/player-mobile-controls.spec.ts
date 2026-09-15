import {expect, test} from "@playwright/test";
import {installPlayerExperienceFixture} from "./player-experience-fixture";

installPlayerExperienceFixture();

test("scrubbing previews without repeatedly seeking and commits on release", async ({page}) => {
  const seek = page.locator("[data-player-seek]");
  await seek.evaluate((element: HTMLInputElement) => {
    for (const value of [30, 40, 55]) {
      element.value = String(value);
      element.dispatchEvent(new Event("input"));
    }
    document.querySelector("video")!.dispatchEvent(new Event("timeupdate"));
  });
  await expect(seek).toHaveAttribute("aria-valuetext", "0:55 of 1:40");
  expect(await page.locator("video").evaluate(v => v.currentTime)).toBe(20);
  await seek.dispatchEvent("change");
  expect(await page.locator("video").evaluate(v => v.currentTime)).toBe(55);
  await seek.press("ArrowRight");
  expect(await page.locator("video").evaluate(v => v.currentTime)).toBe(56);
});

test("canceled mobile scrub returns to the actual playback position", async ({page}) => {
  const seek = page.locator("[data-player-seek]");
  await seek.evaluate((element: HTMLInputElement) => {
    element.value = "80";
    element.dispatchEvent(new Event("input"));
    element.dispatchEvent(new Event("pointercancel"));
  });
  await expect(seek).toHaveValue("20");
  expect(await page.locator("video").evaluate(v => v.currentTime)).toBe(20);
});

test("native caption changes synchronize settings and CC restores the chosen language", async ({page}) => {
  await page.locator("video").evaluate(v => { v.textTracks[1].mode = "showing"; });
  const select = page.locator("[data-subtitles]");
  const cc = page.getByRole("button", {name:"Subtitles", exact:true});
  await expect(select).toHaveValue("1");
  await expect(cc).toHaveAttribute("aria-pressed", "true");
  await cc.click();
  await expect(select).toHaveValue("off");
  await cc.click();
  await expect(select).toHaveValue("1");
  expect(await page.locator("video").evaluate(v => [...v.textTracks].map(t => t.mode))).toEqual(["disabled", "showing"]);
});

test("hiding the page saves mobile progress without pausing playback", async ({page}) => {
  await page.getByRole("button", {name:"Play", exact:true}).first().click();
  const saves = await page.evaluate(() => {
    const requests: {body:string; keepalive:boolean}[] = [];
    window.fetch = async (_url, init) => {
      if (init?.body instanceof URLSearchParams) requests.push({body:init.body.toString(), keepalive:!!init.keepalive});
      return new Response(null, {status:204});
    };
    Object.defineProperty(document, "visibilityState", {configurable:true, value:"hidden"});
    document.dispatchEvent(new Event("visibilitychange"));
    return requests;
  });
  expect(saves).toHaveLength(1);
  expect(new URLSearchParams(saves[0].body).get("seconds")).toBe("20");
  expect(saves[0].keepalive).toBe(true);
  expect(await page.locator("video").evaluate(v => v.paused)).toBe(false);
});

test("transport keys stay scoped and preserve focused seek behavior", async ({page}) => {
 const stage = page.getByRole('region', {name:'Video player'});
 await stage.press('k');
 expect(await page.locator('video').evaluate(v=>v.paused)).toBe(false);
 await stage.press('Space');
 expect(await page.locator('video').evaluate(v=>v.paused)).toBe(true);
 await stage.press('ArrowRight');
 expect(await page.locator('video').evaluate(v=>v.currentTime)).toBe(30);
 await stage.press('ArrowLeft');
 expect(await page.locator('video').evaluate(v=>v.currentTime)).toBe(20);
 await page.evaluate(()=>document.body.insertAdjacentHTML('beforeend','<input aria-label="Search library">'));
 await page.getByRole('textbox', {name:'Search library'}).press('k');
 expect(await page.locator('video').evaluate(v=>v.paused)).toBe(true);
});
test("theater contains keyboard focus and restores the entering control", async ({page}) => {
 await page.getByRole('button',{name:'Theater',exact:true}).click();
 await expect(page.getByRole('dialog',{name:'Video player'})).toBeVisible();
 for(let i=0;i<25;i++) {
  await page.keyboard.press('Tab');
  expect(await page.evaluate(()=>!!document.activeElement?.closest('.media-stage'))).toBe(true);
 }
 await page.keyboard.press('Escape');
 await expect(page.getByRole('button',{name:'Theater',exact:true})).toBeFocused();
 expect(await page.locator('.chapters').evaluate(node=>node.inert)).toBe(false);
});
test("no subtitle tracks disables CC with an explanation", async ({page}) => {
 await page.locator('[data-subtitles]').evaluate(select=>{ select.innerHTML='<option value="off">Off</option>'; });
 await expect(page.getByRole('button',{name:'No subtitles available'})).toBeDisabled();
});
test("rejected Picture-in-Picture requests report recovery instead of silence", async ({page}) => {
 await page.locator('video').evaluate(video=>Object.defineProperty(video,'requestPictureInPicture',{configurable:true,value:()=>Promise.reject(new Error('unavailable'))}));
 await page.getByRole('button',{name:'Picture-in-Picture',exact:true}).click();
 await expect(page.getByText('Picture-in-Picture could not open. Start the video, then try again.')).toBeVisible();
});
