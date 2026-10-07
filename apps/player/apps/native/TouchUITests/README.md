# iOS playback regression

Use a disposable iOS simulator. The fixture automatically approves its test device.
It serves one generated video and supports pending, loaded, and failed playback.
It binds only to `127.0.0.1:4281`. Stop the fixture after testing.

From the repository root, generate the test video:

```sh
mkdir -p .verification/ios-playback
ffmpeg -f lavfi -i 'testsrc2=size=320x180:rate=24' \
  -f lavfi -i 'sine=frequency=440:sample_rate=48000' -t 60 \
  -c:v libx264 -preset ultrafast -crf 32 -pix_fmt yuv420p \
  -c:a aac -movflags +faststart .verification/ios-playback/video.mp4
python3 apps/player/apps/native/scripts/ios-playback-fixture.py \
  .verification/ios-playback/video.mp4
```

In another terminal, run the UI suite. Replace `SIMULATOR_ID` with the disposable
simulator's ID. Use a new result path for each run.

```sh
xcodebuild -project apps/player/apps/native/Kinosail.xcodeproj \
  -scheme Kinosail-iOS-Touch \
  -destination 'platform=iOS Simulator,id=SIMULATOR_ID' \
  -derivedDataPath apps/player/apps/native/.build/ios-simulator \
  -jobs 2 -parallel-testing-enabled NO ARCHS=arm64 ONLY_ACTIVE_ARCH=YES \
  CODE_SIGNING_ALLOWED=YES CODE_SIGN_IDENTITY=- \
  KINOSAIL_SOURCE_REVISION="$(git rev-parse HEAD)" \
  -resultBundlePath .verification/ios-playback/result.xcresult test
```

Retain the result bundle and generated video. Record the source revision, any
uncommitted patch, exact command, Xcode version, simulator model/runtime, video
SHA-256, and result alongside them. The bundle includes screenshots of pending,
loaded, and failed playback. Run on phone and tablet; the suite also rotates
playback to landscape and sends gestures through the displayed app.
