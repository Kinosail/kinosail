import AVFoundation
import Foundation

extension PlaybackEngine {
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
