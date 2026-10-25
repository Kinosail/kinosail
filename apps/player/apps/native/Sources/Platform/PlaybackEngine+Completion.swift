import AVFoundation
import OSLog

private let completionLog = Logger(subsystem: "com.kinosail.player", category: "completion")

extension PlaybackEngine {
    func didEnd(attempt: UUID, player: AVPlayer, nativeItem: AVPlayerItem, restored: Bool = false) async {
        guard generation == attempt, self.player === player, player.currentItem === nativeItem else {
            completionLog.debug("operation=native-end outcome=stale session=\(attempt.uuidString, privacy: .public)")
            return
        }
        guard !completed, let item = currentItem else {
            completionLog.debug("operation=native-end outcome=duplicate session=\(attempt.uuidString, privacy: .public)")
            return
        }
        completed = true
        wantsPlayback = false; nativeIntent.playing.withLock { $0 = false }
        player.pause(); isPlaying = false; buffering = false
        saveProgress(watched: true)
        completionLog.info("operation=native-end outcome=completed restored=\(restored) session=\(attempt.uuidString, privacy: .public)")
        onCompleted?(item, source?.nextItemID)
        guard generation == attempt, self.player === player, player.currentItem === nativeItem, completed else { return }
        guard let next = queue.next(automatic: true), let client, let store else { return await advanceEpisode(after: item) }
        do { try await play(next, client: client, store: store) }
        catch {   }
    }
}
