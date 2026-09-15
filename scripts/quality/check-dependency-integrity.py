#!/usr/bin/env python3
"""Reject changes to reviewed executable assets until their hashes are reviewed."""
import hashlib
import json
from pathlib import Path

if __name__ == "__main__" and (Path(__file__).resolve().parents[2] / ".gates-disabled").is_file():
    print("Quality gates are disabled until explicitly enabled (.gates-disabled).")
    raise SystemExit(0)
import subprocess

GOVAD_VERSION = "v0.0.0-20260330155402-74750eabf3a4"
BROWSER_HASHES = {
    "hls.min.js": "6cfad701a61fb8a99add5e84449e64661169b0652bf44ceb2a28465c8817b5f1",
    "htmx.min.js": "71ea67185bfa8c98c39d31717c6fce5d852370fcdfd129db4543774d3145c0de",
}
MODEL_HASHES = {
    "vad.go": "73fc381fe750e5afc8be27c123682635bc344ac83602784cb834bbd7f939a9b8",
    "model/silero_vad.bin": "b8df2e6e32753b7aa47ab59571b0d9d0b490a223f8dc9118bb388efeaec6f8e3",
}


def check_file(path: Path, expected: str) -> None:
    if not path.is_file() or path.stat().st_size > (8 << 20):
        raise ValueError("reviewed artifact must be a regular file of at most 8 MiB")
    with path.open("rb") as source:
        digest = hashlib.file_digest(source, "sha256").hexdigest()
    if digest != expected:
        raise ValueError(f"unreviewed third-party artifact: {path}")


def verify(repo: Path, module: dict) -> None:
    if module.get("Version") != GOVAD_VERSION or module.get("Replace"):
        raise ValueError("govad version or replacement needs source/model review")
    directory = Path(module["Dir"])
    for name, digest in MODEL_HASHES.items():
        check_file(directory / name, digest)
    for app in ("player", "subtitles"):
        for name, digest in BROWSER_HASHES.items():
            check_file(repo / "apps" / app / "internal/server/static" / name, digest)


def main() -> None:
    repo = Path(__file__).resolve().parents[2]
    result = subprocess.run(
        ["go", "list", "-m", "-json", "github.com/zserge/govad"],
        cwd=repo / "apps/subtitles", capture_output=True, text=True, check=True,
    )
    verify(repo, json.loads(result.stdout))
    print("reviewed browser assets and govad source/model hashes verified")


if __name__ == "__main__":
    main()
