# Native R07 preview journey

This fixture starts the actual Kinosail Subtitles Server HTTP adapter on an ephemeral loopback port. It serves real assets, inspect/preview API responses, and playable media. The browser uses no response routing.

The installation is disposable and authentication is disabled, as in the existing public API fixtures. Providers have no credentials. Background jobs are disabled. FFmpeg is replaced with `/usr/bin/false`; this journey uses an existing clip and performs no encoding. An outer fixture guard permits GET and preview POST only. It rejects Save and every other mutation.

1. Create an explicit fixture directory beneath `apps/subtitles/.verification/`.
2. Copy the approved fictional clip into `media/R07 Example.mp4`. It must contain 396,548 bytes and SHA256 `9dbd85e7863d921e209978c8349992e5f8505b0d7ffa468c37b8caf97af3f0a4`.
3. Place the four-cue synthetic SRT from `initialSubtitle` in `media/R07 Example.en.srt`. Its SHA256 is `eb3bde14cacc59d517305b4dcc93e1b72b7eab099666a36b1e216583e9372aa4`.
4. Run the commands below from `apps/subtitles/`, with the explicit fixture path and tested revision. Reserve the shared build/browser slot first.

```sh
cp engineering/qa/2026-10-04-cue-pairing/native-fixture/main.go.txt .verification/r07-pairing/native-fixture.go
GOMAXPROCS=2 go build -p 1 -o .verification/r07-pairing/native-fixture .verification/r07-pairing/native-fixture.go
.verification/r07-pairing/native-fixture -fixture "$PWD/.verification/r07-pairing/20261004-native" -revision "<tested revision>"
```

In another terminal, from `apps/subtitles/e2e/`:

```sh
KINOSAIL_SUBTITLE_PAIRING_NATIVE_MANIFEST="$PWD/../.verification/r07-pairing/20261004-native/ready.json" \
KINOSAIL_E2E_VIDEO=off KINOSAIL_BROWSER_WORKERS=1 \
node node_modules/@playwright/test/cli.js test subtitle-pairing-native.spec.ts --project=chromium --workers=1
```

The runner produces `ready.json` with the Server URL, media ID, revision, hashes, and test boundary. Stop this fixture process with Ctrl-C after verification. It also stops after two minutes. A clean shutdown produces `integrity.json` only after checking unchanged installed data and no recovery copy. All fixture files are preserved.

The checked-in Go source is a manual QA artifact template. Copying it into the ignored fixture directory keeps this evidence-only command out of the shipped Server package set. Compile and lint the copied source explicitly when changing it; application tests and gates remain unchanged.

The browser verifies direct playback, source/proposed correspondence, removed/merged labels, each side's seek target, safe imported text, phone/desktop overflow, accessibility, and read/preview-only requests. Its attachments and screenshots identify the actual revision and runtime. Container, authentication, hosted-browser, cross-browser, Nox, and device checks remain separate evidence boundaries.
