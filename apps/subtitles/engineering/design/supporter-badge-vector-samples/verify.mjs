import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const directory = fileURLToPath(new URL(".", import.meta.url));
const files = (await readdir(directory)).filter((file) => /^\d{2}-.*\.svg$/.test(file)).sort();
const index = await readFile(`${directory}index.html`, "utf8");

assert.equal(files.length, 10, "expected ten SVG badge sets");
assert.equal((index.match(/<figure tabindex="0">/g) || []).length, 10, "each scrollable comparison needs keyboard focus");

for (const file of files) {
  const svg = await readFile(`${directory}${file}`, "utf8");
  assert.match(index, new RegExp(`src="${file.replace(".", "\\.")}"`), `${file} is missing from the gallery`);
  assert.match(svg, /^<svg[^>]+viewBox="0 0 1800 740"/);
  assert.match(svg, /<title id="title">/);
  assert.match(svg, /<desc id="desc">/);
  assert.doesNotMatch(svg, /<(?:image|foreignObject|filter|mask)\b/i, `${file} contains non-geometry content`);
  assert.doesNotMatch(svg, /(?:\b(?:href|src)=|data:image|url\()/i, `${file} contains an external or embedded asset`);

  const badges = [...svg.matchAll(/data-badge="([^"]+)"/g)].map((match) => match[1]);
  assert.equal(badges.length, 20, `${file} must contain twenty badges`);
  assert.equal(new Set(badges).size, 20, `${file} badge identifiers must be unique`);

  for (const family of ["living", "patron"]) {
    const familyLabel = family === "living" ? "Living Standard" : "Patron Order";
    const labels = [...svg.matchAll(new RegExp(`aria-label="${familyLabel} level [^"]+"`, "g"))];
    assert.equal(labels.length, 10, `${file} must label each ${family} badge`);

    const art = [...svg.matchAll(new RegExp(`<g class="${family}" data-badge="[^"]+"[^>]*>\\s*<g aria-hidden="true">([\\s\\S]*?)<text class="rank"`, "g"))]
      .map((match) => match[1]);
    assert.equal(art.length, 10, `${file} must expose ten ${family} art groups`);
    assert.equal(new Set(art).size, 10, `${file} ${family} rank geometry must remain distinct`);

    for (let rank = 1; rank <= 10; rank += 1) {
      assert.ok(badges.includes(`${family}-${rank}`), `${file} omits ${family} level ${rank}`);
    }
  }
}

console.log(`Verified ${files.length} vector sets and ${files.length * 20} badge samples.`);
