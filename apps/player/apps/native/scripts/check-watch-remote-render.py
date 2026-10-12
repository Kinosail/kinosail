#!/usr/bin/env python3
"""Check visible Watch Remote state in a captured simulator PNG (render evidence only)."""
import argparse
import datetime
import hashlib
import json
import pathlib
import subprocess
import tempfile

SWIFT = r'''import Foundation
import Vision
let request = VNRecognizeTextRequest()
request.recognitionLevel = .accurate
request.recognitionLanguages = ["en-US"]
request.usesLanguageCorrection = false
try VNImageRequestHandler(url: URL(fileURLWithPath: CommandLine.arguments[1])).perform([request])
let lines = (request.results ?? []).compactMap { $0.topCandidates(1).first?.string }
let data = try JSONSerialization.data(withJSONObject: lines)
print(String(decoding: data, as: UTF8.self))
'''

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--image", type=pathlib.Path, required=True)
    parser.add_argument("--state", choices=["connecting", "unavailable", "empty", "playing"], required=True)
    parser.add_argument("--title", help="Known synthetic title for the playing fixture")
    parser.add_argument("--output", type=pathlib.Path, required=True)
    parser.add_argument("--ocr", type=pathlib.Path, help="Prebuilt local Vision OCR helper")
    args = parser.parse_args()
    if args.state == "playing" and not args.title:
        parser.error("playing requires a known synthetic --title")
    if not args.image.is_file() or not 0 < args.image.stat().st_size <= 16 * 1024 * 1024:
        parser.error("image must be a nonempty PNG no larger than 16 MiB")
    if args.image.read_bytes()[:8] != b"\x89PNG\r\n\x1a\n":
        parser.error("image must have a PNG signature")
    args.output.mkdir(mode=0o700, parents=True, exist_ok=False)
    with tempfile.TemporaryDirectory(prefix="kinosail-watch-render-") as temporary:
        root = pathlib.Path(temporary)
        executable = args.ocr
        if executable is None:
            source = root / "ocr.swift"
            source.write_text(SWIFT)
            executable = root / "ocr"
            compiled = subprocess.run(["xcrun", "swiftc", str(source), "-o", str(executable)], capture_output=True)
            (args.output / "compiler-private.log").write_bytes(compiled.stdout + compiled.stderr)
            if compiled.returncode:
                raise SystemExit("Watch render OCR compilation failed; inspect private compiler evidence")
        result = subprocess.run([str(executable.resolve()), str(args.image.resolve())], capture_output=True)
        (args.output / "ocr-private.json").write_bytes(result.stdout)
        (args.output / "ocr-private.stderr").write_bytes(result.stderr)
        if result.returncode:
            raise SystemExit("Watch render OCR failed; inspect private evidence")
    recognized = json.loads(result.stdout)
    if isinstance(recognized, dict):
        recognized = [line["text"] for line in recognized["lines"]]
    text = " ".join(" ".join(recognized).split())
    # Vision misreads this fixed product token on the small Watch; captured pixels confirm it.
    text = text.replace("IPhone", "iPhone").replace("¡Phone", "iPhone")
    checks = {"hasVisibleContent": bool(text), "hasAppHeading": "Kinosail" in text or "This iPhone" in text}
    if args.state == "connecting":
        checks.update(connecting="Connecting" in text and "iPhone" in text,
                      oneConnectingMessage=text.count("Connecting") == 1,
                      noEmptyClaim="Nothing playing" not in text, noStartTitleGuidance="Start a title" not in text)
    elif args.state == "unavailable":
        checks.update(unavailable="iPhone unavailable" in text, recovery="paired iPhone" in text,
                      noEmptyClaim="Nothing playing" not in text, noStartTitleGuidance="Start a title" not in text)
    elif args.state == "empty":
        checks.update(empty="Nothing playing" in text, guidance="Start a title" in text,
                      noConnecting="Connecting" not in text, noUnavailable="iPhone unavailable" not in text)
    else:
        checks.update(title=args.title in text, noEmptyClaim="Nothing playing" not in text,
                      noConnecting="Connecting" not in text, noUnavailable="iPhone unavailable" not in text)
    receipt = {"state": args.state, "passed": all(checks.values()), "checks": checks,
               "utc": datetime.datetime.now(datetime.timezone.utc).isoformat(),
               "imageSHA256": hashlib.sha256(args.image.read_bytes()).hexdigest(),
               "checkerSHA256": hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),
               "boundary": "Visible render assertion only; captured simulator state does not prove transport, controls, Health or physical behavior"}
    (args.output / "safe-receipt.json").write_text(json.dumps(receipt, indent=2) + "\n")
    print(json.dumps(receipt))
    return 0 if receipt["passed"] else 1

if __name__ == "__main__":
    raise SystemExit(main())
