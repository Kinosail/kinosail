// Pixel evidence supplements the native media clock: fixture video is colored and moving.
export function decodedMotion(a, b) {
  if (!a || !b || a.width !== b.width || a.height !== b.height || a.width < 80 || a.height < 80 || a.width > 4096 || a.height > 4096 || a.data.length !== a.width * a.height * 4 || b.data.length !== b.width * b.height * 4) return false;
  let sampled = 0, color = 0, moving = 0;
  // Center excludes system bars, title, timeline and ordinary playback controls.
  for (let y = Math.floor(a.height * .40); y < a.height * .60; y += 2) for (let x = Math.floor(a.width * .35); x < a.width * .65; x += 2) {
    const p = (y * a.width + x) * 4;
    sampled++;
    const rgb = [b.data[p], b.data[p + 1], b.data[p + 2]];
    if (Math.max(...rgb) - Math.min(...rgb) > 50 && Math.max(...rgb) > 80) color++;
    if (Math.abs(a.data[p] - b.data[p]) + Math.abs(a.data[p + 1] - b.data[p + 1]) + Math.abs(a.data[p + 2] - b.data[p + 2]) > 80) moving++;
  }
  return color / sampled > .05 && moving / sampled > .02;
}
