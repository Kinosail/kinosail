import {createRequire} from 'node:module';
import {mkdtempSync, readFileSync, rmSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';

const require = createRequire(join(process.argv[2], 'package.json'));
const {chromium, webkit} = require('@playwright/test');
const page = readFileSync(new URL('./dialog-backdrop.html', import.meta.url), 'utf8');
const rows = [];
for (const [engine, type] of [['webkit', webkit], ['chromium', chromium]]) {
  for (const backdrop of ['backdrop-filter:blur(5px)', 'backdrop-filter:none']) {
    for (let sample = 0; sample < 3; sample++) {
      const cache = mkdtempSync(join(tmpdir(), 'mesa-cache-'));
      const browser = await type.launch({env: {...process.env, MESA_SHADER_CACHE_DIR: cache}});
      const tab = await browser.newPage({viewport: {width: 390, height: 650}});
      await tab.setContent(page.replace('BACKDROP', backdrop));
      await tab.waitForTimeout(500);
      const first = await tab.evaluate(() => window.measure());
      const second = await tab.evaluate(() => window.measure());
      rows.push({engine, version: browser.version(), backdrop, sample, firstOpenMaxGapMs: Math.round(first.max), secondOpenMaxGapMs: Math.round(second.max)});
      await browser.close();
      rmSync(cache, {recursive: true, force: true});
    }
  }
}
console.log(JSON.stringify(rows, null, 1));
