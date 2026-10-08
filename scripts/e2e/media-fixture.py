#!/usr/bin/env python3
"""Bounded original audio, EPUB, comic and photo data for the owned process fixture."""
from pathlib import Path
import struct
import subprocess
import sys
import zlib
import zipfile

if len(sys.argv) != 2 or not sys.argv[1] or len(sys.argv[1]) > 4096:
    print("invalid disposable media root", file=sys.stderr)
    raise SystemExit(2)
raw_root = Path(sys.argv[1])
root = raw_root.resolve()
if (not raw_root.is_absolute() or not root.is_dir() or raw_root.is_symlink()
        or root.name != "Movies" or root.parent.name != "media"
        or not root.parent.parent.name.startswith("kinosail-e2e-player-")):
    print("invalid disposable media root", file=sys.stderr)
    raise SystemExit(2)
album = root / "E2E Album"
album.mkdir()
for number, word in ((1, "One"), (2, "Two")):
    subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-f", "lavfi",
        "-i", f"sine=frequency={220 * number}:sample_rate=48000:duration=8", "-c:a", "aac",
        "-b:a", "96k", "-metadata", f"title=E2E Track {word}", "-metadata", "artist=Kinosail E2E",
        "-metadata", "album=E2E Album", "-metadata", f"track={number}/2",
        str(album / f"E2E Track {word}.m4a")], check=True)

def chunk(kind, data):
    return struct.pack(">I", len(data)) + kind + data + struct.pack(">I", zlib.crc32(kind + data))
# One filter byte per row, followed by RGB pixels.
pixels = b"".join(b"\0" + b"".join(bytes([x * 4, y * 5, 120]) for x in range(64)) for y in range(48))
image = b"\x89PNG\r\n\x1a\n" + chunk(b"IHDR", struct.pack(">IIBBBBB", 64, 48, 8, 2, 0, 0, 0))
image += chunk(b"IDAT", zlib.compress(pixels)) + chunk(b"IEND", b"")
(root / "E2E Photo.png").write_bytes(image)
with zipfile.ZipFile(root / "E2E Comic.cbz", "w") as archive:
    archive.writestr("01.png", image)
    archive.writestr("02.png", image)
with zipfile.ZipFile(root / "E2E EPUB.epub", "w") as archive:
    archive.writestr("mimetype", "application/epub+zip")
    archive.writestr("META-INF/container.xml",
        '<container><rootfiles><rootfile full-path="OEBPS/content.opf"/></rootfiles></container>')
    archive.writestr("OEBPS/content.opf",
        '<package><manifest><item id="one" href="one.xhtml" media-type="application/xhtml+xml"/>'
        '<item id="two" href="two.xhtml" media-type="application/xhtml+xml"/></manifest>'
        '<spine><itemref idref="one"/><itemref idref="two"/></spine></package>')
    for word in ("one", "two"):
        archive.writestr(f"OEBPS/{word}.xhtml",
            f'<html><body><h1>E2E chapter {word}</h1><p>Original disposable reader content.</p></body></html>')

# Real AAC M4B chapters share the existing generated track; no added toolchain.
metadata = root / "chapters.ffmeta"
metadata.write_text(';FFMETADATA1\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=0\nEND=4000\ntitle=E2E first chapter\n[CHAPTER]\nTIMEBASE=1/1000\nSTART=4000\nEND=24000\ntitle=E2E second chapter\n')
subprocess.run(["ffmpeg", "-hide_banner", "-loglevel", "error", "-y", "-stream_loop", "2", "-i",
    str(album / "E2E Track One.m4a"), "-i", str(metadata), "-map_metadata", "1",
    "-map_chapters", "1", "-metadata", "title=E2E Audiobook", "-c", "copy",
    str(root / "E2E Audiobook.m4b")], check=True)
metadata.unlink()
# Original one-page PDF with complete xref offsets; rendering is a browser boundary.
stream = (b"BT /F1 12 Tf 20 150 Td (Original E2E PDF content) Tj ET "
          b"1 0 0 rg 40 40 40 40 re f 0 1 1 rg 100 40 40 40 re f "
          b"0 0 1 rg 40 90 40 40 re f 1 1 0 rg 100 90 40 40 re f")
objects = [b"<< /Type /Catalog /Pages 2 0 R >>",
    b"<< /Type /Pages /Kids [3 0 R] /Count 1 >>",
    b"<< /Type /Page /Parent 2 0 R /MediaBox [0 0 240 180] /Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>",
    b"<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica >>",
    b"<< /Length " + str(len(stream)).encode() + b" >>\nstream\n" + stream + b"\nendstream"]
pdf = b"%PDF-1.4\n"
offsets = []
for number, value in enumerate(objects, 1):
    offsets.append(len(pdf))
    pdf += str(number).encode() + b" 0 obj\n" + value + b"\nendobj\n"
xref = len(pdf)
pdf += b"xref\n0 6\n0000000000 65535 f \n"
pdf += b"".join(f"{offset:010d} 00000 n \n".encode() for offset in offsets)
pdf += f"trailer\n<< /Size 6 /Root 1 0 R >>\nstartxref\n{xref}\n%%EOF\n".encode()
(root / "E2E PDF.pdf").write_bytes(pdf)
