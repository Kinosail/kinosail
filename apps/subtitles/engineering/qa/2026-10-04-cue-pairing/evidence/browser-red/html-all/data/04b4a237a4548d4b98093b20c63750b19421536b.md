# Instructions

- Following Playwright test failed.
- Explain why, be concise, respect Playwright best practices.
- Provide a snippet of code with the fix, if possible.

# Test info

- Name: subtitle-pairing.spec.ts >> cleanup compares the same dialogue at 1440px
- Location: subtitle-pairing.spec.ts:11:3

# Error details

```
Error: expect(locator).toHaveCount(expected) failed

Locator:  locator('.subtitle-cue-row')
Expected: 3
Received: 4
Timeout:  10000ms

Call log:
  - Expect "toHaveCount" locator('.subtitle-cue-row') with timeout 10000ms
  - waiting for locator('.subtitle-cue-row')
    24 × locator resolved to 4 elements
       - unexpected value "4"

```

# Page snapshot

```yaml
- generic [active] [ref=e1]:
  - link "Skip to content" [ref=e2] [cursor=pointer]:
    - /url: "#main"
  - banner [ref=e3]:
    - link "Kinosail Subtitle library Beta" [ref=e4] [cursor=pointer]:
      - /url: /
      - generic [ref=e5]:
        - text: Kinosail
        - generic [ref=e6]: Subtitle library
      - generic [ref=e7]: Beta
    - navigation "Main navigation" [ref=e8]:
      - heading "Subtitles" [level=2] [ref=e9]
      - link "Overview" [ref=e10] [cursor=pointer]:
        - /url: /?view=summary
      - link "Wanted" [ref=e11] [cursor=pointer]:
        - /url: /?view=wanted
      - link "Library" [ref=e12] [cursor=pointer]:
        - /url: /?view=library
      - link "History" [ref=e13] [cursor=pointer]:
        - /url: /?view=history
    - generic [ref=e14]:
      - link "Support Kinosail" [ref=e15] [cursor=pointer]:
        - /url: /supporter
        - text: Support
        - generic [ref=e16]: Kinosail
      - link "Settings" [ref=e17] [cursor=pointer]:
        - /url: /settings
  - main [ref=e18]:
    - generic [ref=e20]:
      - link "Back to library" [ref=e21] [cursor=pointer]:
        - /url: /?view=library
      - heading "Arrival" [level=1] [ref=e22]
      - paragraph [ref=e23]: Subtitle inspector · en
    - status [ref=e24]: Preview ready. Compare the text and timing, then save when satisfied.
    - generic [ref=e25]:
      - region [ref=e26]:
        - heading "Video preview" [level=2] [ref=e27]
        - generic "Video with subtitle preview" [ref=e28]
        - paragraph [ref=e29]:
          - text: This video cannot play directly in this browser. Choose a cue below to hear it in context. If this format cannot play here,
          - link "open the compatible player" [ref=e30] [cursor=pointer]:
            - /url: /watch/53ae726daeaed3fa
          - text: .
        - group "Preview track" [ref=e31]:
          - generic [ref=e33]:
            - radio "Current" [ref=e34]
            - text: Current
          - generic [ref=e35]:
            - radio "Proposed" [checked] [ref=e36]
            - text: Proposed
        - region [ref=e37]:
          - heading "Quality findings" [level=2] [ref=e38]
          - generic [ref=e39]:
            - paragraph [ref=e40]:
              - text: Source
              - strong [ref=e41]: Local file
            - paragraph [ref=e42]:
              - text: Identity evidence
              - strong [ref=e43]: No verified identity evidence
            - paragraph [ref=e44]:
              - text: Installed role
              - strong [ref=e45]: Dialogue subtitles
            - paragraph [ref=e46]:
              - text: Cues
              - strong [ref=e47]: "2"
            - paragraph [ref=e48]:
              - text: Reading above 20 characters/sec
              - strong [ref=e49]: "0"
            - paragraph [ref=e50]:
              - text: Overlapping cues
              - strong [ref=e51]: "0"
            - paragraph [ref=e52]:
              - text: Lines above 42 characters
              - strong [ref=e53]: "0"
            - paragraph [ref=e54]:
              - text: Timing evidence
              - strong [ref=e55]: manual
            - paragraph [ref=e56]:
              - text: Completeness
              - strong [ref=e57]: Needs a language and dialogue review; timing cannot prove completeness
          - list [ref=e58]:
            - listitem [ref=e59]: Normalized to UTF-8 SRT
            - listitem [ref=e60]: Removed subtitle credits at the edges
            - listitem [ref=e61]: Merged adjacent repeated cues
        - region [ref=e62]:
          - heading "Audio reference" [level=2] [ref=e63]
          - paragraph [ref=e64]: Analyze the audio on this Server to see where dialogue occurs throughout the video.
          - button "Analyze audio" [ref=e65] [cursor=pointer]
          - status
      - region [ref=e66]:
        - group [ref=e67]:
          - generic "Create a local subtitle draft" [ref=e68] [cursor=pointer]
          - option "Read image-based subtitles (OCR)" [selected]
          - option "Transcribe dialogue"
        - heading "Prepare a change" [level=2] [ref=e69]
        - generic [ref=e70]:
          - generic [ref=e71]:
            - text: Subtitle language
            - combobox "Subtitle language" [ref=e72]:
              - option "English (en)" [selected]
              - option "Spanish (es)"
              - option "Spanish (Latin America) (es-419)"
              - option "French (fr)"
              - option "German (de)"
              - option "Portuguese (Brazil) (pt-BR)"
              - option "Portuguese (Portugal) (pt-PT)"
              - option "Italian (it)"
              - option "Dutch (nl)"
              - option "Polish (pl)"
              - option "Russian (ru)"
              - option "Ukrainian (uk)"
              - option "Turkish (tr)"
              - option "Arabic (ar)"
              - option "Persian (fa)"
              - option "Hebrew (he)"
              - option "Hindi (hi)"
              - option "Bangla (bn)"
              - option "Urdu (ur)"
              - option "Indonesian (id)"
              - option "Malay (ms)"
              - option "Vietnamese (vi)"
              - option "Thai (th)"
              - option "Chinese (Simplified) (zh-Hans)"
              - option "Chinese (Traditional) (zh-Hant)"
              - option "Japanese (ja)"
              - option "Korean (ko)"
              - option "Filipino (fil)"
              - option "Filipino (tl)"
              - option "Tamil (ta)"
              - option "Telugu (te)"
              - option "Swahili (sw)"
              - option "Romanian (ro)"
              - option "Czech (cs)"
              - option "Slovak (sk)"
              - option "Hungarian (hu)"
              - option "Bulgarian (bg)"
              - option "Greek (el)"
              - option "Serbian (Cyrillic) (sr-Cyrl)"
              - option "Serbian (Latin) (sr-Latn)"
              - option "Croatian (hr)"
              - option "Bosnian (bs)"
              - option "Slovenian (sl)"
              - option "Macedonian (mk)"
              - option "Albanian (sq)"
              - option "Catalan (ca)"
              - option "Basque (eu)"
              - option "Galician (gl)"
              - option "Swedish (sv)"
              - option "Danish (da)"
              - option "Norwegian Bokmål (nb)"
              - option "Norwegian Nynorsk (nn)"
              - option "Finnish (fi)"
              - option "Icelandic (is)"
              - option "Estonian (et)"
              - option "Latvian (lv)"
              - option "Lithuanian (lt)"
              - option "Georgian (ka)"
              - option "Armenian (hy)"
              - option "Azerbaijani (az)"
              - option "Kazakh (kk)"
              - option "Uzbek (uz)"
              - option "Khmer (km)"
              - option "Burmese (my)"
              - option "Nepali (ne)"
              - option "Sinhala (si)"
              - option "Marathi (mr)"
              - option "Gujarati (gu)"
              - option "Punjabi (pa)"
              - option "Malayalam (ml)"
              - option "Kannada (kn)"
              - option "Amharic (am)"
              - option "Afrikaans (af)"
              - option "Abkhazian (ab)"
              - option "Afar (aa)"
              - option "Akan (ak)"
              - option "Akan (tw)"
              - option "Aragonese (an)"
              - option "Assamese (as)"
              - option "Asturian (ast)"
              - option "Avaric (av)"
              - option "Avestan (ae)"
              - option "Aymara (ay)"
              - option "Bambara (bm)"
              - option "Bashkir (ba)"
              - option "Belarusian (be)"
              - option "Bislama (bi)"
              - option "Breton (br)"
              - option "Cantonese (yue)"
              - option "Cebuano (ceb)"
              - option "Central Kurdish (ckb)"
              - option "Chamorro (ch)"
              - option "Chechen (ce)"
              - option "Chinese (zh)"
              - option "Church Slavic (cu)"
              - option "Chuvash (cv)"
              - option "Cornish (kw)"
              - option "Corsican (co)"
              - option "Cree (cr)"
              - option "Dari (prs)"
              - option "Divehi (dv)"
              - option "Dzongkha (dz)"
              - option "Esperanto (eo)"
              - option "Ewe (ee)"
              - option "Extremaduran (ext)"
              - option "Faroese (fo)"
              - option "Fijian (fj)"
              - option "Fulah (ff)"
              - option "Ganda (lg)"
              - option "Guarani (gn)"
              - option "Haitian Creole (ht)"
              - option "Hausa (ha)"
              - option "Herero (hz)"
              - option "Hiri Motu (ho)"
              - option "Ido (io)"
              - option "Igbo (ig)"
              - option "Interlingua (ia)"
              - option "Interlingue (ie)"
              - option "Inuktitut (iu)"
              - option "Inupiaq (ik)"
              - option "Irish (ga)"
              - option "Javanese (jv)"
              - option "Kalaallisut (kl)"
              - option "Kanuri (kr)"
              - option "Kashmiri (ks)"
              - option "Kikuyu (ki)"
              - option "Kinyarwanda (rw)"
              - option "Komi (kv)"
              - option "Kongo (kg)"
              - option "Kuanyama (kj)"
              - option "Kurdish (ku)"
              - option "Kyrgyz (ky)"
              - option "Lao (lo)"
              - option "Latin (la)"
              - option "Limburgish (li)"
              - option "Lingala (ln)"
              - option "Luba-Katanga (lu)"
              - option "Luxembourgish (lb)"
              - option "Malagasy (mg)"
              - option "Maltese (mt)"
              - option "Manipuri (mni)"
              - option "Manx (gv)"
              - option "Maori (mi)"
              - option "Marshallese (mh)"
              - option "Mongolian (mn)"
              - option "Montenegrin (cnr)"
              - option "Nauru (na)"
              - option "Navajo (nv)"
              - option "Ndonga (ng)"
              - option "North Ndebele (nd)"
              - option "Northern Sami (se)"
              - option "Norwegian Bokmål (no)"
              - option "Nyanja (ny)"
              - option "Occitan (oc)"
              - option "Odia (or)"
              - option "Ojibwa (oj)"
              - option "Oromo (om)"
              - option "Ossetic (os)"
              - option "Pali (pi)"
              - option "Pashto (ps)"
              - option "Portuguese (pt)"
              - option "Portuguese (Mozambique) (pt-MZ)"
              - option "Quechua (qu)"
              - option "Romansh (rm)"
              - option "Rundi (rn)"
              - option "Samoan (sm)"
              - option "Sango (sg)"
              - option "Sanskrit (sa)"
              - option "Santali (sat)"
              - option "Sardinian (sc)"
              - option "Scottish Gaelic (gd)"
              - option "Serbian (sr)"
              - option "Serbo-Croatian (sh)"
              - option "Shona (sn)"
              - option "Sichuan Yi (ii)"
              - option "Sindhi (sd)"
              - option "Somali (so)"
              - option "South Azerbaijani (azb)"
              - option "South Ndebele (nr)"
              - option "Southern Sotho (st)"
              - option "Spanish (Spain) (es-ES)"
              - option "Sundanese (su)"
              - option "Swati (ss)"
              - option "Syriac (syr)"
              - option "Tahitian (ty)"
              - option "Tajik (tg)"
              - option "Tatar (tt)"
              - option "Tetum (tet)"
              - option "Tibetan (bo)"
              - option "Tigrinya (ti)"
              - option "Toki Pona (tok)"
              - option "Tongan (to)"
              - option "Tsonga (ts)"
              - option "Tswana (tn)"
              - option "Turkmen (tk)"
              - option "Uyghur (ug)"
              - option "Venda (ve)"
              - option "Volapük (vo)"
              - option "Walloon (wa)"
              - option "Welsh (cy)"
              - option "Western Frisian (fy)"
              - option "Wolof (wo)"
              - option "Xhosa (xh)"
              - option "Yiddish (yi)"
              - option "Yoruba (yo)"
              - option "Zhuang (za)"
              - option "Zulu (zu)"
          - generic [ref=e73]:
            - text: Import a subtitle file
            - button "Import a subtitle file" [ref=e74]
          - paragraph [ref=e75]: SRT, WebVTT, ASS/SSA, or TTML · up to 4 MB. The original is retained when you save.
          - generic [ref=e76]:
            - text: Subtitle role
            - combobox "Subtitle role" [ref=e77]:
              - option "Dialogue subtitles" [selected]
              - option "Accessibility captions (SDH)"
          - paragraph [ref=e78]: Choose captions only after checking sound descriptions and speaker information. A generated transcript does not automatically include this information.
          - generic [ref=e79]:
            - text: Text encoding
            - combobox "Text encoding" [ref=e80]:
              - option "Automatic · use language when needed" [selected]
              - option "utf-8"
              - option "utf-16le"
              - option "utf-16be"
              - option "windows-1252"
              - option "windows-1250"
              - option "windows-1251"
              - option "windows-1253"
              - option "windows-1254"
              - option "windows-1255"
              - option "windows-1256"
              - option "windows-874"
              - option "shift-jis"
              - option "gb18030"
              - option "big5"
              - option "euc-kr"
          - generic [ref=e81]:
            - checkbox "Synchronize against the video's main dialogue" [ref=e82]
            - text: Synchronize against the video's main dialogue
          - paragraph [ref=e83]: Automatic sync and text cleanup are on for previews. Review the result before saving. If audio cannot be matched, turn off sync or use manual timing.
          - group [ref=e84]:
            - generic "Timing anchors and cleanup" [ref=e85] [cursor=pointer]
            - generic [ref=e86]:
              - text: Shift all cues (seconds)
              - spinbutton "Shift all cues (seconds)" [ref=e87]: "0.75"
            - paragraph [ref=e88]: Positive values show subtitles later. Use anchors to correct drift; choose either a shift or anchors.
            - button "Add timing anchor" [ref=e89] [cursor=pointer]
            - generic [ref=e90]:
              - checkbox "Remove subtitle credits at the beginning and end" [checked] [ref=e91]
              - text: Remove subtitle credits at the beginning and end
            - generic [ref=e92]:
              - checkbox "Merge adjacent repeated cues" [checked] [ref=e93]
              - text: Merge adjacent repeated cues
          - group [ref=e94]:
            - generic "Correct subtitle text" [ref=e95] [cursor=pointer]
          - button "Preview changes" [ref=e96] [cursor=pointer]
        - button "Save reviewed subtitle" [ref=e97] [cursor=pointer]
        - paragraph [ref=e98]: Saving keeps a recovery copy and protects your edit from automatic upgrades.
        - generic [ref=e99]:
          - link "Export SRT" [ref=e100] [cursor=pointer]:
            - /url: http://pairing.test/api/v1/subtitle-library/53ae726daeaed3fa/export?language=en&format=srt
          - link "Export WebVTT" [ref=e101] [cursor=pointer]:
            - /url: http://pairing.test/api/v1/subtitle-library/53ae726daeaed3fa/export?language=en&format=vtt
    - region [ref=e102]:
      - generic [ref=e103]:
        - heading "Before and after" [level=2] [ref=e104]
        - generic [ref=e105]:
          - checkbox "Only cues with findings" [ref=e106]
          - text: Only cues with findings
      - paragraph [ref=e107]: Reading-speed and overlap findings are suggestions; signs, songs, and simultaneous speakers may be intentional.
      - generic [ref=e108]:
        - generic [ref=e109]:
          - generic [ref=e110]:
            - text: Current · cue 1
            - button "0:01.000 → 0:02.000" [ref=e111] [cursor=pointer]
            - paragraph [ref=e112]: Downloaded from www.example.com
            - text: Fast reading speed
          - generic [ref=e113]:
            - text: Proposed · cue 1
            - button "0:03.750 → 0:05.750" [ref=e114] [cursor=pointer]
            - paragraph [ref=e115]: Hello there
        - generic [ref=e116]:
          - generic [ref=e117]:
            - text: Current · cue 2
            - button "0:03.000 → 0:04.000" [ref=e118] [cursor=pointer]
            - paragraph [ref=e119]: Hello there
          - generic [ref=e120]:
            - text: Proposed · cue 2
            - button "0:07.750 → 0:08.750" [ref=e121] [cursor=pointer]
            - paragraph [ref=e122]: A later line
        - generic [ref=e123]:
          - generic [ref=e124]:
            - text: Current · cue 3
            - button "0:04.000 → 0:05.000" [ref=e125] [cursor=pointer]
            - paragraph [ref=e126]: Hello there
          - generic [ref=e127]:
            - text: Proposed · cue 3
            - paragraph [ref=e128]: —
        - generic [ref=e129]:
          - generic [ref=e130]:
            - text: Current · cue 4
            - button "0:07.000 → 0:08.000" [ref=e131] [cursor=pointer]
            - paragraph [ref=e132]: A later line
          - generic [ref=e133]:
            - text: Proposed · cue 4
            - paragraph [ref=e134]: —
      - navigation "Cue pages" [ref=e135]:
        - generic [ref=e136]: 4 of 4 cues shown
```

# Test source

```ts
  1  | import AxeBuilder from "@axe-core/playwright";
  2  | import { expect, type Page, type TestInfo } from "@playwright/test";
  3  | 
  4  | export async function reviewSubtitlePairing(page: Page, testInfo: TestInfo) {
  5  |   await expect(page.locator("#inspector-status")).toHaveText("Current subtitle loaded. Preview a change before saving.");
  6  |   await page.locator('input[name="automaticSync"]').uncheck();
  7  |   await page.locator('#subtitle-edit-form details').first().locator("summary").click();
  8  |   await page.locator('input[name="offset"]').fill("0.75");
  9  |   await page.getByRole("button", { name: "Preview changes", exact: true }).click();
  10 |   await expect(page.locator("#inspector-status")).toContainText("Preview ready");
  11 |   const rows = page.locator(".subtitle-cue-row");
  12 |   await page.screenshot({ path: testInfo.outputPath("actual-cleanup.png"), fullPage: true });
> 13 |   await expect(rows).toHaveCount(3);
     |                      ^ Error: expect(locator).toHaveCount(expected) failed
  14 |   await expect(rows.nth(0)).toContainText("Removed during cleanup");
  15 |   await expect(rows.nth(0).locator("div").first()).toContainText("Downloaded from www.example.com");
  16 |   await expect(rows.nth(1)).toContainText("Merged");
  17 |   expect(await rows.nth(1).locator("div").first().locator("p").allTextContents()).toEqual(["Hello there", "Hello there"]);
  18 |   expect(await rows.nth(1).locator("div").nth(1).locator("p").allTextContents()).toEqual(["Hello there"]);
  19 |   await expect(rows.nth(1)).toContainText("Current · cues 2, 3");
  20 |   await expect(rows.nth(1)).toContainText("Proposed · cue 1");
  21 |   expect(await rows.nth(2).locator("p").allTextContents()).toEqual(["A later line", "A later line"]);
  22 |   await rows.nth(2).locator("div").nth(1).getByRole("button", { name: "Seek Proposed cue 2: 0:07.750 to 0:08.750", exact: true }).click();
  23 |   await expect(page.locator('input[name="preview-track"][value="proposed"]')).toBeChecked();
  24 |   await expect.poll(() => page.locator("video").evaluate(video => video.currentTime)).toBe(6.75);
  25 |   await expect(page.locator("video")).toBeFocused();
  26 |   await page.screenshot({ path: testInfo.outputPath("paired-cleanup.png"), fullPage: true });
  27 |   expect(await page.evaluate(() => document.documentElement.scrollWidth - innerWidth)).toBeLessThanOrEqual(1);
  28 |   expect((await new AxeBuilder({ page }).withTags(["wcag2a", "wcag2aa", "wcag21aa"]).analyze()).violations).toEqual([]);
  29 | 
  30 |   await page.locator('input[name="offset"]').fill("0");
  31 |   await page.locator('input[name="file"]').setInputFiles({ name: "translation.srt", mimeType: "application/x-subrip", buffer: Buffer.from("1\n00:00:03,000 --> 00:00:05,000\nBonjour <i>ami</i>\n") });
  32 |   await page.getByRole("button", { name: "Preview changes", exact: true }).click();
  33 |   await expect(rows).toHaveCount(5);
  34 |   await expect(rows.nth(0)).toContainText("Correspondence not verified");
  35 |   await expect(rows.nth(4).locator("p").last()).toHaveText("Bonjour <i>ami</i>");
  36 |   await expect(rows.locator("i")).toHaveCount(0);
  37 |   await expect(rows.nth(4)).toContainText("Correspondence not verified");
  38 |   await page.screenshot({ path: testInfo.outputPath("unpaired-import.png"), fullPage: true });
  39 | }
  40 | 
```