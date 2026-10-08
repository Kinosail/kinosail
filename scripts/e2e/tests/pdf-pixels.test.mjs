import { test } from 'node:test';
import assert from 'node:assert/strict';
import { PNG } from 'pngjs';
import { pdfDocumentPixels } from '../pdf-pixels.mjs';

function picture({ order = ['blue', 'yellow', 'red', 'cyan'], missing = false, wrongGeometry = false } = {}) {
  const image = new PNG({ width: 480, height: 360 });
  image.data.fill(255);
  const colors = { blue: [0, 0, 255], yellow: [255, 255, 0], red: [255, 0, 0], cyan: [0, 255, 255] };
  for (const [i, name] of order.entries()) {
    if (missing && i === 3) continue;
    const startX = 80 + (i % 2) * (wrongGeometry ? 190 : 120), startY = 100 + Math.floor(i / 2) * 100;
    for (let y = startY; y < startY + 80; y++) for (let x = startX; x < startX + 80; x++) {
      if (x >= image.width || y >= image.height) continue;
      const at = (y * image.width + x) * 4;
      image.data.set([...colors[name], 255], at);
    }
  }
  return PNG.sync.write(image);
}
test('actual pixel oracle recognizes only the original PDF four-square geometry', () => {
  assert.equal(pdfDocumentPixels(picture()).documentPattern, true);
});
for (const [name, buffer] of [
  ['blank viewport', PNG.sync.write(new PNG({ width: 480, height: 360 }))],
  ['toolbar or incomplete document', picture({ missing: true })],
  ['wrong order', picture({ order: ['red', 'yellow', 'blue', 'cyan'] })],
  ['wrong geometry', picture({ wrongGeometry: true })],
  ['PDF source bytes are not rendered pixels', Buffer.from('%PDF-1.4')],
  ['oversized input', Buffer.alloc(8 * 1024 * 1024 + 1)],
]) test('rejects ' + name, () => assert.throws(() => pdfDocumentPixels(buffer)));
