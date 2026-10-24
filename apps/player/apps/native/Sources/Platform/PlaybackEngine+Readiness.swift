import AVFoundation
import Foundation
import OSLog

private let readinessLog = Logger(subsystem: "com.kinosail.player", category: "playback-start")

extension PlaybackEngine {
    func beginPlayback(since started: ContinuousClock.Instant, session: UUID) {
        if nativeIntent.playing.withLock({ $0 }) ?? wantsPlayback { player?.play() }
        let elapsed = started.duration(to: .now).components
        let milliseconds = elapsed.seconds * 1000 + elapsed.attoseconds / 1_000_000_000_000_000
        readinessLog.info("operation=playback-start outcome=ready mode=\(self.usingCompatibility ? "compatible" : "direct", privacy: .public) ms=\(milliseconds, privacy: .public) session=\(session.uuidString, privacy: .public)")
    }

    func awaitReadiness(of item: AVPlayerItem, attempt: UUID, installation: UUID) async throws {
        let (statuses, readiness) = AsyncStream.makeStream(of: AVPlayerItem.Status.self, bufferingPolicy: .bufferingNewest(1))
        let observation = item.observe(\.status, options: [.initial, .new]) { observed, _ in readiness.yield(observed.status) }
        let deadline = Task {
            try? await Task.sleep(for: .seconds(45))
            guard !Task.isCancelled else { return }
            readiness.finish()
        }
        for await status in statuses where status != .unknown { break }
        deadline.cancel()
        observation.invalidate()
        try check(attempt, installation: installation)
        guard item.status != .unknown else { throw URLError(.timedOut) }
    }
}
