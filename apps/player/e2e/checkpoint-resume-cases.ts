import {expect, test, type Locator, type Page} from "@playwright/test";

// Public progress writes seed a saved position; rendered watch HTML and the real
// video decoder must agree on re-entry. No player implementation is replaced.
export function registerResumeCheckpoints(flows: {
  phase: string;
  openMovie: (page: Page) => Promise<{watch: string; media: Locator; id: string; duration: number}>;
}) {
  for (const state of ["near-end", "zero", "completed"] as const) {
    test(`saved ${state} progress re-enters the actual movie at the expected position`, {tag: "@smoke"}, async ({page}, info) => {
      test.skip(flows.phase !== "candidate", "historical replay is separate from saved-position semantics");
      const {watch, id, duration} = await flows.openMovie(page);
      await page.getByRole("link", {name: "Library", exact: true}).click();
      await expect(page).toHaveURL(url => url.pathname === "/");
      const csrf = await page.locator('meta[name="kinosail-csrf"]').getAttribute("content");
      expect(csrf).toBeTruthy();
      const headers = {Origin: new URL(page.url()).origin, "X-Kinosail-CSRF": csrf!};
      const target = state === "near-end" ? duration - 8 : 0;
      expect(duration).toBeGreaterThan(20);
      const saved = await page.request.put(`/api/v1/items/${id}/progress`, {headers,
        data: {seconds: target, watched: state === "completed", session: `resume-${state}-${info.project.name}`, revision: 1}});
      expect(saved.status()).toBe(200);
      const before = await page.request.get(`/api/v1/items/${id}`);
      expect(before.status()).toBe(200);
      const accepted = (await before.json()).item.progress;
      expect(accepted.seconds || 0).toBeCloseTo(target, 3);
      expect(Boolean(accepted.watched)).toBe(state === "completed");
      // Rejected positions cannot poison the next rendered resume or stored state.
      for (const seconds of [-1, "not-a-position"]) {
        const invalid = await page.request.put(`/api/v1/items/${id}/progress`, {headers,
          data: {seconds, session: "invalid-resume-position", revision: 1}});
        expect(invalid.status()).toBe(400);
        const unchanged = await page.request.get(`/api/v1/items/${id}`);
        expect(unchanged.status()).toBe(200);
        expect((await unchanged.json()).item.progress).toEqual(accepted);
      }
      await page.goto(watch);
      const media = page.locator("video");
      await expect(media).toHaveAttribute("data-start", String(target));
      if (state === "near-end") {
        expect(new URL((await media.getAttribute("src"))!, page.url()).hash).toBe(`#t=${target}`);
      } else {
        expect(new URL((await media.getAttribute("src"))!, page.url()).hash).toBe("");
      }
      await expect.poll(() => media.evaluate(video => video.readyState)).toBeGreaterThanOrEqual(2);
      const frame = await media.evaluate((video: HTMLVideoElement) => new Promise<number>((resolve, reject) => {
        const timeout = setTimeout(() => reject(new Error("decoded saved-position frame deadline")), 8000);
        video.requestVideoFrameCallback((_, metadata) => {clearTimeout(timeout); resolve(metadata.mediaTime);});
        void video.play().catch(reject);
      }));
      expect(frame).toBeGreaterThanOrEqual(target - 0.1);
      expect(frame).toBeLessThan(target + 2);
      await media.evaluate(video => video.pause());
      await info.attach("saved-position-decoded-reentry", {body: JSON.stringify({state, duration, target,
        decodedSeconds: frame, invalidWritesRejectedWithoutMutation: true,
        boundary: "Real generated direct H264 video and public Go state; not physical/native-HLS evidence."}), contentType: "application/json"});
    });
  }
}
