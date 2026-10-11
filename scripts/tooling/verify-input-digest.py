#!/usr/bin/env python3
"""Hash one verification stage's inputs without a process for every file."""
import hashlib
import os
import sys


def digest():
    # A new key format cannot reuse entries written by the old shell hasher.
    key = hashlib.sha256(b'kinosail-verify-input-v2\0')
    for argument in sys.argv[1:]:
        value = os.fsencode(argument)
        key.update(len(value).to_bytes(8, 'big') + value)
    key.update(b'\0inputs\0')
    for line in sys.stdin.buffer:
        path = line.removesuffix(b'\n')
        key.update(len(path).to_bytes(8, 'big') + path)
        if not os.path.isfile(path):
            key.update(b'missing\0')
            continue
        content = hashlib.sha256()
        with open(path, 'rb') as source:
            for block in iter(lambda: source.read(1024 * 1024), b''):
                content.update(block)
        key.update(b'file\0' + content.digest())
    return key.hexdigest()


if __name__ == '__main__':
    print(digest())
