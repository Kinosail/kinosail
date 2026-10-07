#!/usr/bin/env python3
"""Generate a fictional two-track album in a new disposable media directory."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import struct
import wave
import zlib


def cover(path, color):
    def chunk(kind, value):
        return (struct.pack('>I', len(value)) + kind + value +
                struct.pack('>I', zlib.crc32(kind + value)))
    pixels = b''.join(b'\x00' + bytes(min(255, value + y) for value in color) * 64
                      for y in range(64))
    path.write_bytes(b'\x89PNG\r\n\x1a\n' +
                     chunk(b'IHDR', struct.pack('>IIBBBBB', 64, 64, 8, 2, 0, 0, 0)) +
                     chunk(b'IDAT', zlib.compress(pixels)) + chunk(b'IEND', b''))


def prepare(directory):
    # Refuse an existing directory or symlink; never replace media or proof files.
    directory.mkdir()
    tracks = [('01 Lantern', 'Lantern Start', 'Aster Vale', 440, (10, 50, 90)),
              ('02 Copper', 'Copper &lt;Moon&gt; &amp; Harbor', 'Mira Tide', 660, (20, 80, 40))]
    for number, (name, title, artist, frequency, color) in enumerate(tracks, 1):
        with wave.open(str(directory / (name + '.wav')), 'wb') as audio:
            audio.setparams((1, 2, 16000, 0, 'NONE', 'not compressed'))
            audio.writeframes(b''.join(struct.pack('<h', round(4000 * math.sin(
                2 * math.pi * frequency * index / 16000))) for index in range(12 * 16000)))
        (directory / (name + '.nfo')).write_text(
            f'<track><title>{title}</title><artist>{artist}</artist>'
            '<albumartist>Fictional Ensemble</albumartist><album>R08 Fictional Session</album>'
            f'<disc>1</disc><track>{number}</track><year>{2025 + number}</year></track>', encoding='utf-8')
        cover(directory / (name + '.png'), color)
    return {path.name: hashlib.sha256(path.read_bytes()).hexdigest()
            for path in sorted(directory.iterdir())}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path, help='New directory beneath the disposable media root')
    args = parser.parse_args()
    print(json.dumps({'fixture': 'R08 Fictional Session', 'sha256': prepare(args.directory)}, sort_keys=True))
