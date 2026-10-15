#!/usr/bin/env python3
import base64
import hashlib
import hmac
import os
import re
import struct
import sys
import tempfile
import time
from pathlib import Path

STEP = 30
LOCK_STALE_SECONDS = 120
LOCK_WAIT_SECONDS = 600


def code(secret, counter):
    key = base64.b32decode(secret + "=" * (-len(secret) % 8))
    digest = hmac.new(key, struct.pack(">Q", counter), hashlib.sha1).digest()
    offset = digest[19] & 15
    value = struct.unpack(">I", digest[offset:offset + 4])[0] & 0x7FFFFFFF
    return f"{value % 1_000_000:06d}"


def state_path(secret):
    root = Path(os.environ.get("KINOSAIL_TOTP_STATE_DIR") or tempfile.gettempdir())
    return root / f"kinosail-totp-{hashlib.sha256(secret.encode()).hexdigest()[:24]}"


def acquire(lock):
    deadline = time.monotonic() + LOCK_WAIT_SECONDS
    while True:
        try:
            lock.mkdir()
            return
        except FileExistsError:
            try:
                if time.time() - lock.stat().st_mtime > LOCK_STALE_SECONDS:
                    lock.rmdir()
                    continue
            except FileNotFoundError:
                continue
            if time.monotonic() > deadline:
                raise SystemExit("fixture TOTP lock timed out")
            time.sleep(0.05)


def allocate(secret):
    path = state_path(secret)
    lock = path.with_name(path.name + ".lock")
    acquire(lock)
    try:
        try:
            last = int(path.read_text())
        except (FileNotFoundError, ValueError):
            last = 0
        counter = max(int(time.time() // STEP), last + 1)
        while counter > int(time.time() // STEP) + 1:
            time.sleep(STEP - time.time() % STEP + 0.05)
        path.write_text(str(counter))
        return code(secret, counter)
    finally:
        lock.rmdir()


def main(arguments):
    if len(arguments) != 1 or not re.fullmatch(r"[A-Z2-7]{16,128}", arguments[0]):
        print("usage: fixture-totp.py BASE32_SECRET", file=sys.stderr)
        return 2
    print(allocate(arguments[0]))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
