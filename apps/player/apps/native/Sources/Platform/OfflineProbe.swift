#if os(iOS)
import AVFoundation
import Foundation
import OSLog
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
            var failure: NSError?, samples = 0, readerStatus = 0
            do {
                let asset = try asset(url)
                finished.phase("playable")
                guard try await asset.load(.isPlayable) else { throw ClientError.invalidResponse }
                finished.phase("tracks")
                let tracks = try await asset.loadTracks(withMediaType: video ? .video : .audio)
                guard let track = tracks.first else { throw ClientError.invalidResponse }
                if video {
                    finished.phase("geometry")
                    let size = try await track.load(.naturalSize)
                    guard size.width.isFinite, size.height.isFinite, size.width > 0, size.height > 0,
                          size.width <= 8192, size.height <= 8192, size.width * size.height <= 16_777_216 else { throw ClientError.invalidResponse }
                }
                try Task.checkCancellation()
                finished.phase("reader")
                let reader = try AVAssetReader(asset: asset)
                reader.timeRange = CMTimeRange(start: .zero, duration: CMTime(seconds: 2, preferredTimescale: 600))
                let settings: [String: Any] = video
                    ? [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA]
                    : [AVFormatIDKey: kAudioFormatLinearPCM, AVLinearPCMBitDepthKey: 16, AVLinearPCMIsFloatKey: false]
                let output = AVAssetReaderTrackOutput(track: track, outputSettings: settings)
                output.alwaysCopiesSampleData = false
                guard reader.canAdd(output) else { throw ClientError.invalidResponse }
                reader.add(output)
                finished.phase("start-reading")
                guard reader.startReading() else {
                    if let error = reader.error { throw error }
                    throw ClientError.invalidResponse
                }
                defer { reader.cancelReading() }
                finished.phase("sample")
                if let sample = output.copyNextSampleBuffer() {
                    samples = CMSampleBufferGetNumSamples(sample)
                    playable = video ? CMSampleBufferGetImageBuffer(sample) != nil : CMSampleBufferGetNumSamples(sample) > 0 && CMSampleBufferGetDataBuffer(sample) != nil
                }
                readerStatus = reader.status.rawValue
                failure = reader.error as NSError?
                finished.phase("complete")
            } catch { failure = error as NSError; playable = false }
            finished.finish(playable, error: failure, samples: samples, readerStatus: readerStatus)
        }
        Task.detached {
            try? await Task.sleep(for: .seconds(20))
            if finished.finish(false, cause: "deadline") { worker.cancel() }
        }
    }
}
private final class ProbeCompletion: Sendable {
    private static let log = Logger(subsystem: "com.kinosail.player", category: "offline-probe")
    private let state = Mutex((finished: false, phase: "asset"))
    private let started = Date()
    private let completion: @Sendable (Bool) -> Void
    init(_ completion: @escaping @Sendable (Bool) -> Void) { self.completion = completion }
    func phase(_ value: String) { state.withLock { $0.phase = value } }
    @discardableResult func finish(_ value: Bool, cause: String = "worker", error: NSError? = nil, samples: Int = 0, readerStatus: Int = 0) -> Bool {
        let phase = state.withLock { state -> String? in
            guard !state.finished else { return nil }
            state.finished = true
            return state.phase
        }
        guard let phase else { return false }
        let domain = error.map { ["NSCocoaErrorDomain", "NSPOSIXErrorDomain", "NSOSStatusErrorDomain", "AVFoundationErrorDomain", "CoreMediaErrorDomain"].contains($0.domain) ? $0.domain : "other" } ?? "none"
        let code = error?.code ?? 0, elapsed = Date().timeIntervalSince(started)
        Self.log.notice("Offline probe playable=\(value) cause=\(cause, privacy: .public) phase=\(phase, privacy: .public) elapsed=\(elapsed) samples=\(samples) reader=\(readerStatus) errorDomain=\(domain, privacy: .public) errorCode=\(code)")
        completion(value)
        return true
    }
}
#endif
