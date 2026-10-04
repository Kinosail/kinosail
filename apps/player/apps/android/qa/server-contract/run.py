"""Replay real Server/Kotlin contracts without Gradle, emulators, or media encoding."""

import argparse
import datetime
import hashlib
import json
import os
import pathlib
import select
import shutil
import subprocess
import time


fixture = pathlib.Path(__file__).resolve().parent
android = fixture.parents[1]
app = android.parents[1]
repo = app.parents[1]
parser = argparse.ArgumentParser(description=__doc__)
parser.add_argument("output", type=pathlib.Path)
parser.add_argument("--server-binary", required=True, type=pathlib.Path)
parser.add_argument("--java-home", default=os.environ.get("JAVA_HOME"), required=not os.environ.get("JAVA_HOME"))
parser.add_argument("--sdk-root", default=os.environ.get("ANDROID_HOME"), required=not os.environ.get("ANDROID_HOME"))
parser.add_argument("--gradle-cache", type=pathlib.Path, default=pathlib.Path.home() / ".gradle/caches/modules-2/files-2.1")
options = parser.parse_args()
output = options.output.resolve()
output.mkdir(parents=True, exist_ok=False)
started = datetime.datetime.now(datetime.UTC).isoformat()
clock = time.monotonic()
commands = []
java = str(pathlib.Path(options.java_home) / "bin/java")


def jar(group, module, version):
    files = list((options.gradle_cache / group / module / version).glob("*/*.jar"))
    if len(files) != 1:
        raise RuntimeError(f"Expected one cached {module} {version} jar; found {len(files)}")
    return str(files[0])


compiler = [jar("org.jetbrains.kotlin", module, "2.4.20") for module in
            ["kotlin-compiler-embeddable", "kotlin-stdlib", "kotlin-script-runtime", "kotlin-daemon-embeddable"]]
compiler += [jar("org.jetbrains.kotlin", "kotlin-reflect", "1.6.10"),
             jar("org.jetbrains.kotlinx", "kotlinx-coroutines-core-jvm", "1.11.0"),
             jar("org.jetbrains", "annotations", "13.0")]
runtime = [jar("org.jetbrains.kotlin", "kotlin-stdlib", "2.4.20"),
           jar("org.jetbrains.kotlinx", "kotlinx-serialization-core-jvm", "1.11.0"),
           jar("org.jetbrains.kotlinx", "kotlinx-serialization-json-jvm", "1.11.0"),
           jar("com.fasterxml.jackson.core", "jackson-core", "2.22.3"),
           jar("junit", "junit", "4.13.2"), jar("org.hamcrest", "hamcrest-core", "1.3")]
platforms = sorted((pathlib.Path(options.sdk_root) / "platforms").glob("android-37*/android.jar"))
if not platforms:
    raise RuntimeError("Installed Android Platform 37 is required")
runtime.append(str(platforms[-1]))
core = android / "app/src/main/java/com/kinosail/player/core"
sources = [core / f"{name}.kt" for name in ["ServerAddress", "ServerApi", "StrictJson", "PlaybackApi",
           "PlaybackCapabilities", "MediaTimeline", "CatalogApi", "WatchProgress", "ProgressApi", "NetworkDiagnostic"]]
tests = android / "app/src/test/java/com/kinosail/player/core"
sources += [tests / "PlaybackApiTest.kt", tests / "CatalogPageLimitTest.kt", fixture / "main.kt"]


def run(command, name, **kwargs):
    commands.append(command)
    result = subprocess.run(command, capture_output=True, text=True, timeout=120, **kwargs)
    (output / name).write_text(result.stdout + result.stderr)
    return result


compile_result = run([java, "-Xmx768m", "-cp", os.pathsep.join(compiler),
                      "org.jetbrains.kotlin.cli.jvm.K2JVMCompiler", "-no-stdlib", "-no-reflect", "-jvm-target", "17",
                      "-classpath", os.pathsep.join(runtime), "-d", str(output / "classes"),
                      *map(str, sources)], "compile.log")
if compile_result.returncode:
    raise RuntimeError(compile_result.stdout + compile_result.stderr)
runtime.insert(0, str(output / "classes"))
unit = run([java, "-Xmx512m", "-cp", os.pathsep.join(runtime), "org.junit.runner.JUnitCore",
            "com.kinosail.player.core.PlaybackApiTest", "com.kinosail.player.core.CatalogPageLimitTest"], "junit.log")
print(unit.stdout)
media = output / "media"
media.mkdir()
movie = android / "app/src/androidTest/assets/parity-video.mp4"
for index in range(1, 201):
    destination = media / f"Fictional Movie {index:03}.mp4"
    try:
        os.link(movie, destination)
    except OSError:
        shutil.copyfile(movie, destination)
server_command = [str(options.server_binary.resolve()), str(media), str(output / "data"),
                  shutil.which("ffprobe"), shutil.which("ffmpeg")]
commands.append(server_command)
with (output / "server.log").open("w") as log:
    server = subprocess.Popen(server_command, stdout=subprocess.PIPE, stderr=log, text=True)
    try:
        if not select.select([server.stdout], [], [], 30)[0]:
            raise RuntimeError("Loopback Server startup exceeded 30 seconds")
        origin = server.stdout.readline().strip()
        if not origin.startswith("http://127.0.0.1:"):
            raise RuntimeError("Expected a loopback Server origin")
        journey = run([java, "-Xmx512m", "-cp", os.pathsep.join(runtime),
                       "com.kinosail.player.core.MainKt", origin, str(output)], "journey.log")
        print(journey.stdout)
    finally:
        server.terminate()
        try:
            server.wait(timeout=10)
        except subprocess.TimeoutExpired:
            server.kill()
            server.wait(timeout=5)
tracked = sources + [fixture / "run.py", movie, options.server_binary.resolve()]
hashes = {str(path.relative_to(repo)) if path.is_relative_to(repo) else path.name:
          hashlib.sha256(path.read_bytes()).hexdigest() for path in tracked}
for name in ["server-playback.json", "server-library-200.json", "server-history-200.json", "android-result.json", "compile.log", "junit.log", "journey.log"]:
    path = output / name
    if path.exists():
        hashes[name] = hashlib.sha256(path.read_bytes()).hexdigest()
receipt = {"startedUTC": started, "elapsedSeconds": time.monotonic() - clock,
           "revision": subprocess.check_output(["git", "rev-parse", "HEAD"], cwd=repo, text=True).strip(),
           "commands": commands, "hashes": hashes, "junitExitCode": unit.returncode, "journeyExitCode": journey.returncode,
           "boundary": "Real loopback Go Server and production Kotlin HTTP client/decoders on JVM; Android SDK stubs; no UI, codec, emulator, physical device, Nox, or production request"}
(output / "receipt.json").write_text(json.dumps(receipt, indent=2))
raise SystemExit(1 if unit.returncode or journey.returncode else 0)
