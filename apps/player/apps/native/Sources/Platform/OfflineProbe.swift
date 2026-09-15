#if os(iOS)
import AVFoundation
import Foundation
import Synchronization

/// Decodes one local sample through the same AVFoundation stack as the player.
/// Unsupported originals remain unavailable offline with a compatible-format remedy.
enum OfflineProbe {
    /// Private storage names have no media extension. Identify bounded file bytes
    /// for both verification and playback; never let a saved playlist open URLs.
    static func asset(_ url: URL) throws -> AVURLAsset {
        guard url.isFileURL, url.resolvingSymlinksInPath().standardizedFileURL == url.standardizedFileURL else { throw ClientError.invalidResponse }
        let file = try FileHandle(forReadingFrom: url)
        defer { try? file.close() }
        let type = try contentType(file.read(upToCount: 64) ?? Data())
        return AVURLAsset(url: url, options: [AVURLAssetOverrideMIMETypeKey: type,
            AVURLAssetReferenceRestrictionsKey: AVAssetReferenceRestrictions.forbidAll.rawValue])
    }

    static func contentType(_ data: Data) throws -> String {
        let bytes = Array(data)
        guard (12...64).contains(bytes.count) else { throw ClientError.invalidResponse }
        func matches(_ value: String, at offset: Int = 0) -> Bool {
            Array(bytes.dropFirst(offset).prefix(value.utf8.count)) == Array(value.utf8)
        }
        if matches("ftyp", at: 4) || matches("moov", at: 4) || matches("mdat", at: 4) || matches("wide", at: 4) { return "video/mp4" }
        if matches("RIFF") && matches("WAVE", at: 8) { return "audio/wav" }
        if matches("FORM") && (matches("AIFF", at: 8) || matches("AIFC", at: 8)) { return "audio/aiff" }
        if matches("fLaC") { return "audio/flac" }
        if matches("caff") { return "audio/x-caf" }
        if matches("ID3") || (bytes[0] == 0xff && bytes[1] & 0xe0 == 0xe0 && bytes[1] & 0x06 != 0) { return "audio/mpeg" }
        if bytes[0] == 0xff && bytes[1] & 0xf6 == 0xf0 { return "audio/aac" }
        throw ClientError.invalidResponse
    }

    static func check(_ url: URL, video: Bool, completion: @escaping @Sendable (Bool) -> Void) {
        let finished = ProbeCompletion(completion)
        let worker = Task.detached(priority: .utility) {
            var playable = false
            do {
                let asset = try asset(url)
                guard try await asset.load(.isPlayable) else { throw ClientError.invalidResponse }
                let tracks = try await asset.loadTracks(withMediaType: video ? .video : .audio)
                guard let track = tracks.first else { throw ClientError.invalidResponse }
                if video {
                    let size = try await track.load(.naturalSize)
                    guard size.width.isFinite, size.height.isFinite, size.width > 0, size.height > 0,
                          size.width <= 8192, size.height <= 8192, size.width * size.height <= 16_777_216 else { throw ClientError.invalidResponse }
                }
                try Task.checkCancellation()
                let reader = try AVAssetReader(asset: asset)
                reader.timeRange = CMTimeRange(start: .zero, duration: CMTime(seconds: 2, preferredTimescale: 600))
                let settings: [String: Any] = video
                    ? [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA]
                    : [AVFormatIDKey: kAudioFormatLinearPCM, AVLinearPCMBitDepthKey: 16, AVLinearPCMIsFloatKey: false]
                let output = AVAssetReaderTrackOutput(track: track, outputSettings: settings)
                output.alwaysCopiesSampleData = false
                guard reader.canAdd(output) else { throw ClientError.invalidResponse }
                reader.add(output)
                guard reader.startReading() else { throw ClientError.invalidResponse }
                defer { reader.cancelReading() }
                if let sample = output.copyNextSampleBuffer() {
                    playable = video ? CMSampleBufferGetImageBuffer(sample) != nil : CMSampleBufferGetNumSamples(sample) > 0 && CMSampleBufferGetDataBuffer(sample) != nil
                }
            } catch { playable = false }
            finished.finish(playable)
        }
        Task.detached {
            try? await Task.sleep(for: .seconds(20))
            if finished.finish(false) { worker.cancel() }
        }
    }
}
private final class ProbeCompletion: Sendable {
    private let finished = Mutex(false)
    private let completion: @Sendable (Bool) -> Void
    init(_ completion: @escaping @Sendable (Bool) -> Void) { self.completion = completion }
    @discardableResult func finish(_ value: Bool) -> Bool {
        let first = finished.withLock { state in if state { return false }; state = true; return true }
        if first { completion(value) }
        return first
    }
}
#endif
