"""Generate a separate fictional60-second album for saved35 queue admission."""
import argparse
import hashlib
import json
import math
from pathlib import Path
import struct
import wave


def prepare(directory):
    directory.mkdir()  # Refuse existing media, symlinks and prior evidence.
    for number, title in enumerate(('Queue Intent Alpha', 'Queue Intent Beta'), 1):
        stem = f'{number:02d} {title}'
        with wave.open(str(directory / (stem + '.wav')), 'wb') as audio:
            audio.setparams((1, 2, 16000, 0, 'NONE', 'not compressed'))
            audio.writeframes(b''.join(struct.pack('<h', round(2000 * math.sin(
                2 * math.pi * (330 + number * 110) * index / 16000))) for index in range(60 * 16000)))
        (directory / (stem + '.nfo')).write_text(f'<track><title>{title}</title>'
            '<artist>Fictional Queue Ensemble</artist><album>Queue Intent Long Session</album>'
            f'<disc>1</disc><track>{number}</track><year>2026</year></track>', encoding='utf-8')
    return {path.name: hashlib.sha256(path.read_bytes()).hexdigest() for path in sorted(directory.iterdir())}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directory', type=Path)
    args = parser.parse_args()
    print(json.dumps({'fixture': 'Queue Intent Long Session', 'sha256': prepare(args.directory)}, sort_keys=True))
