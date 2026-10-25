import OSLog

private let episodeLog = Logger(subsystem: "com.kinosail.player", category: "episodes")
@MainActor private var automaticEpisodes = 0

extension PlaybackEngine {
    static let automaticEpisodeLimit = 3

    static func viewerActed() { automaticEpisodes = 0 }

    func finishIntoNextEpisode() async {
        guard !completed, let item = currentItem else { return }
        completed = true
        wantsPlayback = false; nativeIntent.playing.withLock { $0 = false }
        player?.pause(); isPlaying = false; buffering = false
        saveProgress(watched: true)
        episodeLog.info("operation=auto-next outcome=credits-skipped")
        onCompleted?(item, source?.nextItemID)
        await advanceEpisode(after: item)
    }

    func advanceEpisode(after item: MediaItem) async {
        guard item.kind == .video, let nextID = source?.nextItemID, let client, let store else { return }
        guard automaticEpisodes < Self.automaticEpisodeLimit else {
            episodeLog.info("operation=auto-next outcome=paused-for-viewer limit=\(Self.automaticEpisodeLimit)")
            return
        }
        automaticEpisodes += 1
        do {
            try await play(client.item(id: nextID), client: client, store: store)
            episodeLog.info("operation=auto-next outcome=started count=\(automaticEpisodes)")
        } catch is CancellationError {
        } catch {
            episodeLog.warning("operation=auto-next outcome=failed class=\(String(describing: type(of: error)), privacy: .public)")
        }
    }
}
