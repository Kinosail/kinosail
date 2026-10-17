#!/usr/bin/env bash
set -euo pipefail
out="${1:?output directory}"
mkdir -p "$out"
render="$out/render-credits.swift"
cat >"$render" <<'EOF'
import AppKit
let episode = CommandLine.arguments[2]
let lines = (1...70).map { "Role \($0) ........ Episode \(episode) Person \($0 * (Int(episode) ?? 1))" }
let width = 320, height = 180 + lines.count * 34
let image = NSImage(size: NSSize(width: width, height: height))
image.lockFocus()
NSColor.black.setFill()
NSRect(x: 0, y: 0, width: width, height: height).fill()
let style = NSMutableParagraphStyle()
style.alignment = .center
let attributes: [NSAttributedString.Key: Any] = [.font: NSFont(name: "Arial", size: 20) ?? NSFont.systemFont(ofSize: 20), .foregroundColor: NSColor.white, .paragraphStyle: style]
for (index, line) in lines.enumerated() {
    NSString(string: line).draw(in: NSRect(x: 0, y: height - 90 - (index + 1) * 34, width: width, height: 30), withAttributes: attributes)
}
image.unlockFocus()
let rep = NSBitmapImageRep(data: image.tiffRepresentation!)!
try! rep.representation(using: .png, properties: [:])!.write(to: URL(fileURLWithPath: CommandLine.arguments[1]))
EOF
for episode in 1 2 3; do
  seed=$((episode * 7919))
  swift "$render" "$out/credits-$episode.png" "$episode"
  melody() { echo "0.3*sin(2*PI*t*$1*pow(2,floor(mod(t*$2,8))*$3/12))+0.15*sin(4*PI*t*$1*pow(2,floor(mod(t*$2,8))*$3/12))+0.08*sin(6*PI*t*$1*pow(2,floor(mod(t*$2,8))*$3/12))"; }
  ffmpeg -hide_banner -loglevel error -y \
    -f lavfi -i "testsrc=size=320x180:rate=25" \
    -f lavfi -i "life=size=320x180:rate=25:seed=$seed:mold=20:life_color=#9a9a9a:death_color=#2a2a2a,boxblur=2" \
    -loop 1 -framerate 25 -i "$out/credits-$episode.png" \
    -f lavfi -i "aevalsrc='$(melody 220 2 2)':s=44100" \
    -f lavfi -i "aevalsrc='$(melody $((180 + episode * 50)) $((episode + 2)) $((episode + 2)))':s=44100" \
    -f lavfi -i "aevalsrc='$(melody 330 3 3)':s=44100" \
    -f lavfi -i "aevalsrc='$(melody $((110 + episode * 30)) 1 $((episode + 4)))':s=44100" \
    -filter_complex "[0:v]trim=0:15,setpts=PTS-STARTPTS[intro]; \
      [1:v]trim=0:505,setpts=PTS-STARTPTS[story]; \
      [0:v]trim=0:20,setpts=PTS-STARTPTS,negate[outro]; \
      [2:v]crop=320:180:0:'min(ih-180,t*36)',trim=0:60,setpts=PTS-STARTPTS[credits]; \
      [3:a]atrim=0:15,asetpts=PTS-STARTPTS[aintro]; \
      [4:a]atrim=0:505,asetpts=PTS-STARTPTS[astory]; \
      [5:a]atrim=0:20,asetpts=PTS-STARTPTS[aoutro]; \
      [6:a]atrim=0:60,asetpts=PTS-STARTPTS[acredits]; \
      [intro][aintro][story][astory][outro][aoutro][credits][acredits]concat=n=4:v=1:a=1[v][a]" \
    -map "[v]" -map "[a]" -c:v libx264 -preset ultrafast -crf 28 -pix_fmt yuv420p -c:a aac -b:a 96k -movflags +faststart \
    "$out/episode-$episode.mp4"
done
shasum -a 256 "$out"/episode-*.mp4 "$out"/credits-*.png >"$out/checksums.txt"
