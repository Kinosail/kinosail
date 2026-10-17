import struct
import sys

from PIL import Image, ImageDraw, ImageFont


def segment(kind, pts, payload):
    return b"PG" + struct.pack(">IIBH", pts, pts, kind, len(payload)) + payload


def rle(image):
    width, height = image.size
    pixels = image.load()
    out = bytearray()
    for y in range(height):
        x = 0
        while x < width:
            color = pixels[x, y]
            run = 1
            while x + run < width and pixels[x + run, y] == color and run < 16383:
                run += 1
            if color != 0 and run <= 2:
                out += bytes([color]) * run
            elif color == 0:
                out += b"\x00" + (bytes([run]) if run < 64 else bytes([0x40 | run >> 8, run & 0xFF]))
            else:
                out += b"\x00" + (bytes([0x80 | run, color]) if run < 64 else bytes([0xC0 | run >> 8, run & 0xFF, color]))
            x += run
        out += b"\x00\x00"
    return bytes(out)


def bitmap(lines, font):
    probe = ImageDraw.Draw(Image.new("L", (1, 1)))
    boxes = [probe.textbbox((0, 0), line, font=font, stroke_width=3) for line in lines]
    width = max(box[2] - box[0] for box in boxes) + 16
    step = max(box[3] - box[1] for box in boxes) + 10
    canvas = Image.new("RGB", (width, step * len(lines) + 10), (0, 0, 0))
    draw = ImageDraw.Draw(canvas)
    for index, line in enumerate(lines):
        draw.text((8, 5 + index * step), line, font=font, fill=(255, 255, 255), stroke_width=3, stroke_fill=(0, 0, 255))
    indexed = Image.new("P", canvas.size, 0)
    source, target = canvas.load(), indexed.load()
    for y in range(canvas.size[1]):
        for x in range(canvas.size[0]):
            red, _, blue = source[x, y]
            target[x, y] = 1 if red > 127 else (2 if blue > 127 else 0)
    return indexed


def display(width, height, number, start, image):
    left, top = (width - image.size[0]) // 2, height - image.size[1] - 40
    pcs = struct.pack(">HHBHBBBB", width, height, 0x10, number, 0x80, 0, 0, 1) + struct.pack(">HBBHH", 0, 0, 0, left, top)
    wds = struct.pack(">BBHHHH", 1, 0, left, top, image.size[0], image.size[1])
    pds = struct.pack(">BB", 0, 0) + bytes([0, 16, 128, 128, 0, 1, 235, 128, 128, 255, 2, 16, 128, 128, 255])
    data = rle(image)
    ods = struct.pack(">HBB", 0, 0, 0xC0) + (len(data) + 4).to_bytes(3, "big") + struct.pack(">HH", *image.size) + data
    return segment(0x16, start, pcs) + segment(0x17, start, wds) + segment(0x14, start, pds) + segment(0x15, start, ods) + segment(0x80, start, b"")


def clear(width, height, number, end):
    pcs = struct.pack(">HHBHBBBB", width, height, 0x10, number, 0x00, 0, 0, 0)
    wds = struct.pack(">BBHHHH", 1, 0, 0, 0, width, 64)
    return segment(0x16, end, pcs) + segment(0x17, end, wds) + segment(0x80, end, b"")


def main():
    output, font_path, width, height = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4])
    cues = [line.split("|") for line in sys.stdin.read().splitlines() if line]
    font = ImageFont.truetype(font_path, 44)
    data = bytearray()
    for number, (start, end, *lines) in enumerate(cues):
        data += display(width, height, number * 2, int(float(start) * 90000), bitmap(lines, font))
        data += clear(width, height, number * 2 + 1, int(float(end) * 90000))
    with open(output, "wb") as handle:
        handle.write(data)


main()
