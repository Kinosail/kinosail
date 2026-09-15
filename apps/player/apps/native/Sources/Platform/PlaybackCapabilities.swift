import AVFoundation
import VideoToolbox

struct PlaybackCapabilities: Sendable {
    let videoCodecs: [String]
    let audioCodecs: [String]
    let hdrFormats: [String]
    let maximumAudioChannels: Int

    @MainActor static func detect() throws -> Self {
        var video = ["h264"]
        if VTIsHardwareDecodeSupported(kCMVideoCodecType_HEVC) { video.append("hevc") }
        if VTIsHardwareDecodeSupported(kCMVideoCodecType_AV1) { video.append("av1") }
        var hdr = ["sdr"]
        if video.contains("hevc"), AVPlayer.eligibleForHDRPlayback { hdr += ["hdr10", "hlg"] }
        let channels = AVAudioSession.sharedInstance().maximumOutputNumberOfChannels
        return Self(videoCodecs: video, audioCodecs: ["aac", "mp3", "ac3", "eac3"], hdrFormats: hdr, maximumAudioChannels: channels > 0 ? min(8, channels) : 2)
    }

    var query: String {
        "videoCodecs=\(videoCodecs.joined(separator: ","))&audioCodecs=\(audioCodecs.joined(separator: ","))&hdrFormats=\(hdrFormats.joined(separator: ","))&maxAudioChannels=\(maximumAudioChannels)"
    }
}
