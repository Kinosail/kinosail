const fs = require('node:fs');
const path = require('node:path');

const [, , playwrightModule, directory] = process.argv;
const { chromium } = require(playwrightModule);

(async () => {
  const browser = await chromium.launch();
  const page = await browser.newPage();
  const files = fs.readdirSync(directory).filter((name) => name.endsWith('.svg')).sort();
  for (const name of files) {
    const svg = fs.readFileSync(path.join(directory, name), 'utf8');
    const width = Number(/width="(\d+)"/.exec(svg)[1]);
    const height = Number(/height="(\d+)"/.exec(svg)[1]);
    await page.setViewportSize({ width, height });
    await page.setContent(`<!doctype html><style>html,body{margin:0;background:#000}</style>${svg}`);
    await page.evaluate(() => document.fonts.ready);
    await page.evaluate((limit) => {
      const lines = [...document.querySelectorAll('text[data-fit]')];
      if (!lines.length) return;
      const size = Number(lines[0].getAttribute('font-size'));
      const widest = Math.max(...lines.map((line) => line.getComputedTextLength()));
      const next = Math.min(size, size * limit / widest);
      const y0 = Number(lines[0].dataset.y0);
      lines.forEach((line, index) => {
        line.setAttribute('font-size', next.toFixed(1));
        line.setAttribute('letter-spacing', (next * 0.04).toFixed(1));
        line.setAttribute('y', (y0 + index * next * 1.02).toFixed(0));
      });
      const tag = document.querySelector('text[data-tag]');
      if (tag) tag.setAttribute('y', (y0 + (lines.length - 1) * next * 1.02 + next * 0.8).toFixed(0));
    }, width * 0.86);
    await page.screenshot({ path: path.join(directory, name.replace(/\.svg$/, '.png')), clip: { x: 0, y: 0, width, height } });
  }
  await browser.close();
  console.log(`Rendered ${files.length} images`);
})();
