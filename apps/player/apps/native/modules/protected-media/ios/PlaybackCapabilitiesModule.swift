import ExpoModulesCore
import AVFoundation
import VideoToolbox

public final class PlaybackCapabilitiesModule: Module {
  // Hardware support is a codec-family hint for the recovery rendition, not
  // certification of arbitrary source profiles, Dolby Vision, or passthrough.
  private static let videoCodecs: [String] = {
    var values = ["h264"]
    if VTIsHardwareDecodeSupported(kCMVideoCodecType_HEVC) { values.append("hevc") }
    if #available(iOS 17.0, tvOS 17.0, *), VTIsHardwareDecodeSupported(kCMVideoCodecType_AV1) { values.append("av1") }
    return values
  }()
  public func definition() -> ModuleDefinition {
    Name("PlaybackCapabilities")
    AsyncFunction("getCapabilities") { () -> [String: Any] in
      var hdr = ["sdr"]
      if Self.videoCodecs.contains("hevc") && AVPlayer.eligibleForHDRPlayback {
        hdr += ["hdr10", "hlg"]
      }
      let channels = AVAudioSession.sharedInstance().maximumOutputNumberOfChannels
      return ["videoCodecs": Self.videoCodecs, "audioCodecs": ["aac", "mp3", "ac3", "eac3"],
        "hdrFormats": hdr, "maxAudioChannels": channels > 0 ? min(8, channels) : 2]
    }.runOnQueue(.main)
  }
}
