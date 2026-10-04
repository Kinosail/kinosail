import hashlib
import json
import pathlib
import shutil
import subprocess
import sys

fixture = pathlib.Path(__file__).resolve().parent
app = fixture.parents[1]
repo = app.parents[1]
output = pathlib.Path(sys.argv[1]).resolve()
output.mkdir(parents=True, exist_ok=False)
media = output / "media"
media.mkdir()
commands = []


def run(command, **kwargs):
    commands.append(command)
    return subprocess.run(command, check=True, timeout=180, **kwargs)


run([shutil.which("ffmpeg"), "-nostdin", "-hide_banner", "-loglevel", "error", "-f", "lavfi", "-i", "testsrc2=size=128x72:rate=30", "-t", "2", "-threads", "1", "-c:v", "libx264", "-preset", "ultrafast", "-pix_fmt", "yuv420p", "-movflags", "+faststart", str(media / "Fictional.mp4")])
run(["go", "build", "-p", "1", "-o", str(output / "server"), str(fixture / "server.go")], cwd=app)
core = app / "apps/native/Sources/Core"
sources = sorted(core.glob("*.swift"))
run(["xcrun", "swiftc", "-parse-as-library", "-module-cache-path", str(output / "module-cache"), *map(str, sources), str(fixture / "main.swift"), "-o", str(output / "native")], cwd=repo)
with (output / "server.log").open("w") as log:
    server = subprocess.Popen([str(output / "server"), str(media), str(output / "data"), shutil.which("ffprobe"), shutil.which("ffmpeg")], stdout=subprocess.PIPE, stderr=log, text=True)
    try:
        origin = server.stdout.readline().strip()
        assert origin.startswith("http://127.0.0.1:")
        command = [str(output / "native"), origin, str(output)]
        commands.append(command)
        result = subprocess.run(command, timeout=45, capture_output=True, text=True)
        (output / "native.log").write_text(result.stdout + result.stderr)
    finally:
        server.terminate()
        server.wait(timeout=10)
hashes = {str(path.relative_to(repo)): hashlib.sha256(path.read_bytes()).hexdigest() for path in sources + [fixture / "main.swift", fixture / "server.go"]}
hashes["fixture.mp4"] = hashlib.sha256((media / "Fictional.mp4").read_bytes()).hexdigest()
receipt = {"revision": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(), "commands": commands, "hashes": hashes, "exitCode": result.returncode, "boundary": "Real loopback Player handler and production Swift Core; no physical playback or Nox request"}
(output / "receipt.json").write_text(json.dumps(receipt, indent=2))
print(result.stdout + result.stderr)
sys.exit(result.returncode)
