# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: library-pagination.spec.ts >> all Shows remain reachable across real Server pages at 390px
- Location: library-pagination.spec.ts:18:2

# Error details

```
Error: expect(locator).toHaveText(expected) failed

Locator: locator('#library .show-details h2')
Timeout: 10000ms
- Expected  - 26
+ Received  +  0

  Array [
    "Pagination Show 01",
    "Pagination Show 02",
    "Pagination Show 03",
    "Pagination Show 04",
-   "Pagination Show 05",
-   "Pagination Show 06",
-   "Pagination Show 07",
-   "Pagination Show 08",
-   "Pagination Show 09",
-   "Pagination Show 10",
-   "Pagination Show 11",
-   "Pagination Show 12",
-   "Pagination Show 13",
-   "Pagination Show 14",
-   "Pagination Show 15",
-   "Pagination Show 16",
-   "Pagination Show 17",
-   "Pagination Show 18",
-   "Pagination Show 19",
-   "Pagination Show 20",
-   "Pagination Show 21",
-   "Pagination Show 22",
-   "Pagination Show 23",
-   "Pagination Show 24",
-   "Pagination Show 25",
-   "Pagination Show 26",
-   "Pagination Show 27",
-   "Pagination Show 28",
-   "Pagination Show 29",
-   "Pagination Show 30",
  ]

Call log:
  - Expect "toHaveText" locator('#library .show-details h2') with timeout 10000ms
  - waiting for locator('#library .show-details h2')
    24 × locator resolved to 4 elements

```

# Page snapshot

```yaml
- generic [active] [ref=e1]:
  - link "Skip to content" [ref=e2] [cursor=pointer]:
    - /url: "#main"
  - banner [ref=e3]:
    - link "Kinosail Beta" [ref=e4] [cursor=pointer]:
      - /url: /
      - heading "Kinosail" [level=1] [ref=e5]
      - generic [ref=e6]: Beta
    - search [ref=e7]:
      - generic [ref=e8]: Search library
      - searchbox "Search all libraries" [ref=e9]
    - link "Support Kinosail" [ref=e10] [cursor=pointer]:
      - /url: /supporter
    - navigation "Main navigation" [ref=e11]:
      - link "Home" [ref=e12] [cursor=pointer]:
        - /url: /?view=all
      - link "TV Shows" [ref=e13] [cursor=pointer]:
        - /url: /?view=shows
      - link "Movies" [ref=e14] [cursor=pointer]:
        - /url: /?view=movies
      - link "Search" [ref=e15] [cursor=pointer]:
        - /url: "#library-search"
      - group [ref=e16]:
        - generic "More" [ref=e17] [cursor=pointer]
  - main [ref=e18]:
    - generic [ref=e19]:
      - heading "Shows" [level=2] [ref=e20]
      - paragraph [ref=e21]: Browse, filter, and play directly from this Server.
    - generic [ref=e22]:
      - generic [ref=e23]:
        - strong [ref=e24]: "30"
        - text: items
      - generic [ref=e25]:
        - text: Sort
        - combobox "Sort" [ref=e26]:
          - option "Title" [selected]
          - option "Newest"
          - option "Year"
      - button "Apply" [ref=e27] [cursor=pointer]
    - generic [ref=e30]:
      - article [ref=e31]:
        - link [ref=e32] [cursor=pointer]:
          - /url: /show/7d143ed446b05d69
          - heading "Pagination Show 01" [level=2] [ref=e35]
        - link "Play next · Pagination Show 01 · Pagination Show 01" [ref=e36] [cursor=pointer]:
          - /url: /watch/632a99864d53b361
          - generic [ref=e38]: Play next
      - article [ref=e39]:
        - link [ref=e40] [cursor=pointer]:
          - /url: /show/26c5e08b2bbe8069
          - heading "Pagination Show 02" [level=2] [ref=e43]
        - link "Play next · Pagination Show 02 · Pagination Show 02" [ref=e44] [cursor=pointer]:
          - /url: /watch/dfda5efcac50e300
          - generic [ref=e46]: Play next
      - article [ref=e47]:
        - link [ref=e48] [cursor=pointer]:
          - /url: /show/f18945bbdb192648
          - heading "Pagination Show 03" [level=2] [ref=e51]
        - link "Play next · Pagination Show 03 · Pagination Show 03" [ref=e52] [cursor=pointer]:
          - /url: /watch/505bbb1aae5d7e64
          - generic [ref=e54]: Play next
      - article [ref=e55]:
        - link [ref=e56] [cursor=pointer]:
          - /url: /show/a292e92cf5d9b813
          - heading "Pagination Show 04" [level=2] [ref=e59]
        - link "Play next · Pagination Show 04 · Pagination Show 04" [ref=e60] [cursor=pointer]:
          - /url: /watch/36be3c20abe62a2a
          - generic [ref=e62]: Play next
    - status [ref=e63]: All titles are loaded.
  - form "Language" [ref=e64]:
    - generic [ref=e65]:
      - text: Language
      - combobox "Language" [ref=e66]:
        - option "Automatic (browser)" [selected]
        - option "English"
        - option "Español"
        - option "Deutsch"
        - option "Français"
        - option "Português (Brasil)"
        - option "简体中文"
        - option "Italiano"
        - option "Nederlands"
        - option "Polski"
        - option "Русский"
        - option "日本語"
        - option "한국어"
        - option "العربية"
        - option "Türkçe"
        - option "Українська"
        - option "Português (Portugal)"
        - option "繁體中文"
        - option "Svenska"
        - option "Abkhazian"
        - option "Afrikaans"
        - option "অসমীয়া"
        - option "azərbaycan"
        - option "беларуская"
        - option "български"
        - option "বাংলা"
        - option "বাংলা"
        - option "brezhoneg"
        - option "bosanski"
        - option "català"
        - option "Chamorro"
        - option "کوردیی ناوەندی"
        - option "čeština"
        - option "Cymraeg"
        - option "dansk"
        - option "Divehi"
        - option "Ελληνικά"
        - option "British English"
        - option "American English"
        - option "esperanto"
        - option "español"
        - option "español de México"
        - option "español latinoamericano"
        - option "español"
        - option "eesti"
        - option "euskara"
        - option "فارسی"
        - option "suomi"
        - option "Filipino"
        - option "føroyskt"
        - option "français canadien"
        - option "Gaeilge"
        - option "galego"
        - option "Schwiizertüütsch"
        - option "ગુજરાતી"
        - option "עברית"
        - option "עברית"
        - option "हिन्दी"
        - option "hrvatski"
        - option "Haitian Creole"
        - option "magyar"
        - option "հայերեն"
        - option "Indonesia"
        - option "íslenska"
        - option "Lojban"
        - option "ქართული"
        - option "Taqbaylit"
        - option "қазақ тілі"
        - option "ಕನ್ನಡ"
        - option "kernewek"
        - option "кыргызча"
        - option "Lëtzebuergesch"
        - option "lietuvių"
        - option "latviešu"
        - option "Malagasy"
        - option "Maori"
        - option "македонски"
        - option "മലയാളം"
        - option "монгол"
        - option "मराठी"
        - option "Melayu"
        - option "Malti"
        - option "မြန်မာ"
        - option "norsk bokmål"
        - option "Low German"
        - option "नेपाली"
        - option "nynorsk"
        - option "Occitan"
        - option "ਪੰਜਾਬੀ"
        - option "português"
        - option "română"
        - option "සිංහල"
        - option "slovenčina"
        - option "slovenščina"
        - option "Soomaali"
        - option "shqip"
        - option "српски"
        - option "Kiswahili"
        - option "தமிழ்"
        - option "తెలుగు"
        - option "ไทย"
        - option "ئۇيغۇرچە"
        - option "اردو"
        - option "o‘zbek"
        - option "Tiếng Việt"
        - option "中文"
        - option "繁體中文"
        - option "繁體中文"
        - option "isiZulu"
    - button "Save" [ref=e67] [cursor=pointer]
    - link "Language" [ref=e68] [cursor=pointer]:
      - /url: /language
```

# Test source

```ts
  1  | import { expect, test } from "@playwright/test";
  2  |
  3  | const origin = process.env.KINOSAIL_LIBRARY_BROWSER_URL;
  4  | test.skip(!origin, "requires TestLibraryPaginationBrowserJourney disposable Go Server");
  5  | test.use({ serviceWorkers: "block" });
  6  |
  7  | const showTitles = Array.from({ length: 30 }, (_, position) => `Pagination Show ${String(position + 1).padStart(2, "0")}`);
  8  | const movieTitles = Array.from({ length: 6 }, (_, position) => `Pagination Movie ${String(position + 1).padStart(2, "0")}`);
  9  |
  10 | async function loadAll(page: import("@playwright/test").Page) {
  11 | 	await expect.poll(async () => {
  12 | 		if (await page.locator("[data-library-next]").count()) await page.locator("[data-library-status]").scrollIntoViewIfNeeded();
  13 | 		return page.locator("[data-library-status]").textContent();
  14 | 	}, { timeout: 20_000 }).toBe("All titles are loaded.");
  15 | }
  16 |
  17 | for (const width of [390, 1440]) {
  18 | 	test(`all Shows remain reachable across real Server pages at ${width}px`, async ({ page }, info) => {
  19 | 		await page.setViewportSize({ width, height: 844 });
  20 | 		const response = await page.request.get(`${origin}/api/v1/library?view=shows&limit=4`);
  21 | 		expect(response.status()).toBe(200);
  22 | 		const api = await response.json();
  23 | 		expect(api.total).toBe(30);
  24 | 		expect(api.items).toHaveLength(4);
  25 | 		await page.goto(`${origin}/?view=shows&limit=4`);
  26 | 		await loadAll(page);
> 27 | 		await expect(page.locator("#library .show-details h2")).toHaveText(showTitles);
     |                                                           ^ Error: expect(locator).toHaveText(expected) failed
  28 | 		const links = await page.locator("#library .show-details").evaluateAll(elements => elements.map(element => element.getAttribute("href")));
  29 | 		expect(new Set(links).size).toBe(30);
  30 | 		for (const link of links) expect(link).toMatch(/^\/show\/[a-f0-9]{16}$/);
  31 | 		expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
  32 | 		await page.screenshot({ path: info.outputPath(`shows-loaded-${width}.png`), fullPage: true });
  33 | 	});
  34 | }
  35 |
  36 | test("real Server mixed pages keep both Show and Movie cards", async ({ page }, info) => {
  37 | 	await page.goto(`${origin}/?q=Pagination&limit=4`);
  38 | 	await loadAll(page);
  39 | 	await expect(page.locator('[data-library-group="shows"] h2')).toHaveText(["Shows", ...showTitles]);
  40 | 	await expect(page.locator('[data-library-group="movies"] h2')).toHaveText(["Movies", ...movieTitles]);
  41 | 	await expect(page.locator("#library .card")).toHaveCount(36);
  42 | 	await page.screenshot({ path: info.outputPath("mixed-loaded.png"), fullPage: true });
  43 | });
  44 |
```
