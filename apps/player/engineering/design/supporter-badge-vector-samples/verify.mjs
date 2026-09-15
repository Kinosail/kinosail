import assert from "node:assert/strict";
import { readFile, readdir } from "node:fs/promises";
import { fileURLToPath } from "node:url";

const directory = fileURLToPath(new URL(".", import.meta.url));
const files = (await readdir(directory)).filter((file) => /^\d{2}-.*\.svg$/.test(file)).sort();
const index = await readFile(`${directory}index.html`, "utf8");

assert.equal(files.length, 10, "expected ten SVG badge sets");
assert.equal((index.match(/<figure tabindex="0">/g) || []).length, 10, "each comparison needs keyboard focus");

for (const file of files) {
  const svg = await readFile(`${directory}${file}`, "utf8");
  assert.match(index, new RegExp(`src="${file.replace(".", "\\.")}"`), `${file} is missing from the gallery`);
  assert.match(svg, /^<svg[^>]+viewBox="0 0 1600 1000"/);
  assert.match(svg, /<title id="title">/);
  assert.match(svg, /<desc id="description">/);
  assert.doesNotMatch(svg, /<(?:image|foreignObject|filter|script)\b/i, `${file} contains non-vector content`);
  assert.doesNotMatch(svg, /(?:\b(?:href|src)=|data:image|https?:\/\/(?!www\.w3\.org\/2000\/svg))/i, `${file} contains an asset reference`);

  const paintReferences = [...svg.matchAll(/url\(#([^)]+)\)/g)].map((match) => match[1]);
  for (const id of paintReferences) {
    assert.match(svg, new RegExp(`id="${id.replace(/[.*+?^${}()|[\]\\]/g, "\\$&")}"`), `${file} has a missing vector paint definition`);
  }

  const badges = [...svg.matchAll(/data-badge="level-(\d+)"/g)].map((match) => Number(match[1]));
  assert.deepEqual(badges, [1, 2, 3, 4, 5, 6, 7, 8, 9, 10], `${file} needs ten ordered levels`);
  const shapes = [...svg.matchAll(/data-shape="([^"]+)"/g)].map((match) => match[1]);
  assert.equal(new Set(shapes).size, 10, `${file} needs ten distinct rank silhouettes`);
  const scales = [...svg.matchAll(/data-art-scale="([^"]+)"/g)].map((match) => Number(match[1]));
  assert.ok(scales.every((scale, index) => index === 0 || scale > scales[index - 1]), `${file} must grow with every rank`);
  assert.equal((svg.match(/role="group" aria-label="[^"]+ badge, level \d+"/g) || []).length, 10, `${file} must label each badge`);
}

console.log(`Verified ${files.length} vector sets and ${files.length * 10} badge samples.`);
