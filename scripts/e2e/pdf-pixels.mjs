import { PNG } from 'pngjs';

// Recognize only the four original document squares, never viewer chrome.
/** @param {Buffer} bytes */
export function pdfDocumentPixels(bytes) {
  if (!Buffer.isBuffer(bytes) || bytes.length < 33 || bytes.length > 8 * 1024 * 1024 ||
      bytes.subarray(0, 8).toString('hex') !== '89504e470d0a1a0a' ||
      bytes.readUInt32BE(16) < 160 || bytes.readUInt32BE(20) < 160 ||
      bytes.readUInt32BE(16) > 2560 || bytes.readUInt32BE(20) > 1600) throw Error('invalid bounded PDF screenshot');
  const image = PNG.sync.read(bytes);
  const groups = ['blue', 'yellow', 'red', 'cyan'].map(name => ({ name, count: 0, x1: image.width, x2: 0, y1: image.height, y2: 0 }));
  for (let y = 0; y < image.height; y++) for (let x = 0; x < image.width; x++) {
    const at = (y * image.width + x) * 4;
    const [r, g, b, a] = image.data.subarray(at, at + 4);
    if (a < 250) continue;
    const index = r < 16 && g < 16 && b > 239 ? 0 : r > 239 && g > 239 && b < 16 ? 1 :
      r > 239 && g < 16 && b < 16 ? 2 : r < 16 && g > 239 && b > 239 ? 3 : -1;
    if (index < 0) continue;
    const group = groups[index];
    group.count++; group.x1 = Math.min(group.x1, x); group.x2 = Math.max(group.x2, x);
    group.y1 = Math.min(group.y1, y); group.y2 = Math.max(group.y2, y);
  }
  const squares = groups.map(group => ({ ...group, width: group.x2 - group.x1 + 1,
    height: group.y2 - group.y1 + 1, x: (group.x1 + group.x2) / 2, y: (group.y1 + group.y2) / 2 }));
  const [blue, yellow, red, cyan] = squares;
  const size = blue.width;
  const near = (/** @type {number} */ value, /** @type {number} */ target) => Math.abs(value - target) <= size * 0.12;
  if (!squares.every(s => s.count >= 200 && s.count / (s.width * s.height) >= 0.9 && near(s.width, size) && near(s.height, size)) ||
      !near(yellow.x - blue.x, size * 1.5) || !near(cyan.x - red.x, size * 1.5) ||
      !near(red.y - blue.y, size * 1.25) || !near(cyan.y - yellow.y, size * 1.25) ||
      !near(blue.x, red.x) || !near(yellow.x, cyan.x) || !near(blue.y, yellow.y) || !near(red.y, cyan.y)) throw Error('original PDF document pattern not visibly decoded');
  return { documentPattern: true, width: image.width, height: image.height,
    coloredPixels: squares.map(s => s.count), boundary: 'native viewer document pixels; no text/accessibility or print proof' };
}
