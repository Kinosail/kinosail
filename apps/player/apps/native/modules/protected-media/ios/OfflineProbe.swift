#if os(iOS)
  import Foundation
  import UIKit
  import VLCKit

  /// A file-only, silent decoder probe. This uses the compatibility engine already
  /// shipped by Player, including originals AVFoundation cannot demux. It neither
  /// opens a network URL nor changes the application's audio session.
  enum OfflineProbe {
    static func check(_ url: URL, video: Bool, completion: @escaping (Bool) -> Void) {
      DispatchQueue.main.async {
        guard url.isFileURL, let media = VLCMedia(url: url) else { completion(false); return }
        var finished = false
        var background: UIBackgroundTaskIdentifier = .invalid
        var timer: Timer?
        let player = VLCMediaPlayer(options: ["--quiet", "--aout=dummy", "--vout=dummy", "--no-video-title-show", "--no-sub-autodetect-file"])
        media.addOption(":access=file")
        player.media = media
        player.play()
        let deadline = Date().addingTimeInterval(20)
        let finish: (Bool) -> Void = { decoded in
          guard !finished else { return }; finished = true
          timer?.invalidate(); timer = nil; player.stop(); completion(decoded)
          if background != .invalid {
            UIApplication.shared.endBackgroundTask(background); background = .invalid
          }
        }
        background = UIApplication.shared.beginBackgroundTask(withName: "Verify offline media") { finish(false) }
        timer = Timer.scheduledTimer(withTimeInterval: 0.1, repeats: true) { _ in
          let stats = media.statistics
          let decoded = video ? stats.decodedVideo > 0 : stats.decodedAudio > 0
          if decoded || player.state == .error || Date() >= deadline {
            finish(decoded)
          }
        }
      }
    }
  }
#endif
