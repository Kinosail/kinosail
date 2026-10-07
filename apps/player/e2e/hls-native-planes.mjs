// Lossless native 8-bit 4:2:0 packing. No format or color conversion.
export function packNative420(bytes, format, width, height, layout) {
  const bound = 1024 * 1024;
  if (!(bytes instanceof Uint8Array) || bytes.byteLength > bound ||
      !['I420', 'NV12'].includes(format) ||
      ![width, height].every(n => Number.isSafeInteger(n) && n > 0 && n <= 1024 && n % 2 === 0) ||
      width * height * 1.5 > bound) throw new Error('native_plane_shape');
  const specs = format === 'I420' ? [[width, height], [width / 2, height / 2], [width / 2, height / 2]] :
    [[width, height], [width, height / 2]];
  if (!Array.isArray(layout) || layout.length !== specs.length) throw new Error('native_plane_count');
  const regions = layout.map((plane, n) => {
    const [columns, rows] = specs[n];
    if (!plane || !Number.isSafeInteger(plane.offset) || plane.offset < 0 ||
        !Number.isSafeInteger(plane.stride) || plane.stride < columns || plane.stride > bound) {
      throw new Error('native_plane_layout');
    }
    const end = plane.offset + (rows - 1) * plane.stride + columns;
    if (!Number.isSafeInteger(end) || end > bytes.byteLength) throw new Error('native_plane_extent');
    return [plane.offset, end];
  });
  for (let a = 0; a < regions.length; a++) for (let b = a + 1; b < regions.length; b++) {
    if (regions[a][0] < regions[b][1] && regions[b][0] < regions[a][1]) {
      throw new Error('native_plane_overlap');
    }
  }
  const packed = new Uint8Array(width * height * 1.5);
  for (let row = 0; row < height; row++) {
    const begin = layout[0].offset + row * layout[0].stride;
    packed.set(bytes.subarray(begin, begin + width), row * width);
  }
  const chroma = width * height / 4;
  for (let row = 0; row < height / 2; row++) for (let column = 0; column < width / 2; column++) {
    const destination = width * height + row * width / 2 + column;
    if (format === 'I420') {
      packed[destination] = bytes[layout[1].offset + row * layout[1].stride + column];
      packed[destination + chroma] = bytes[layout[2].offset + row * layout[2].stride + column];
    } else {
      const begin = layout[1].offset + row * layout[1].stride + column * 2;
      packed[destination] = bytes[begin];
      packed[destination + chroma] = bytes[begin + 1];
    }
  }
  return packed;
}
