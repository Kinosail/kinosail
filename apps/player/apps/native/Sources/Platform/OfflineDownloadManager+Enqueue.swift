#if os(iOS)
import Foundation

extension OfflineDownloadManager {
    nonisolated static func validate(item: MediaItem, quality: DownloadQuality, tracks: DownloadTrackSelection?) throws {
        guard [.video, .music, .audiobook].contains(item.kind), item.kind == .video ? quality != .audio : [.original, .audio].contains(quality),
              quality != .original || tracks == nil, item.kind == .video || tracks == nil else { throw ClientError.invalidInput("Choose a download format supported by this title.") }
        _ = try tracks?.validated()
    }

    func enqueue(item: MediaItem, quality: DownloadQuality, tracks: DownloadTrackSelection? = nil, client: ServerClient) async throws {
        try Self.validate(item: item, quality: quality, tracks: tracks)
        guard !operation, let scope, let storage, catalog.records.count < 50 else { throw ClientError.invalidInput("Wait for the current operation, or remove a download before adding another.") }
        let attempt = generation
        try await performEnqueue {
            let preferences = try await enqueuePreferences(client: client, scope: scope, attempt: attempt)
            try await enqueueOwned(item: item, quality: quality, tracks: tracks, client: client, scope: scope, storage: storage, preferences: preferences, attempt: attempt)
        }
    }

    private func enqueuePreferences(client: ServerClient, scope: String, attempt: UUID) async throws -> MediaPreferences {
        let access = try await client.downloadAuthorization()
        try check(attempt)
        guard access.scope == scope else { throw ClientError.http(403) }
        _ = try await client.downloadIdentity()
        try check(attempt)
        let preferences = try await client.mediaPreferences()
        try check(attempt)
        return preferences
    }

    private func enqueueOwned(item: MediaItem, quality: DownloadQuality, tracks: DownloadTrackSelection?, client: ServerClient,
                              scope: String, storage: OfflineCatalogStore, preferences: MediaPreferences, attempt: UUID) async throws {
        try check(attempt)
        try Self.validate(item: item, quality: quality, tracks: tracks)
        guard catalog.records.count < 50 else { throw ClientError.invalidInput("Remove a download before adding another.") }
        let expected = try OfflineRecord(item: item, jobID: String(repeating: "0", count: 16), quality: quality, tracks: tracks)
        guard !catalog.records.contains(where: { $0.key == expected.key }) else { throw ClientError.invalidInput("This download is already on the device. Open Downloads to resume or remove it.") }
        // Persistable metadata and settings are validated before Server preparation.
        let prepared = try await client.prepareDownload(itemID: item.id, quality: quality, tracks: tracks)
        try check(attempt)
        guard prepared.state != .failed else { throw ClientError.invalidInput(prepared.error ?? "The Server could not prepare this title.") }
        let record = try OfflineRecord(item: item, jobID: prepared.id, quality: quality, tracks: tracks)
        var next = catalog; next.preferences = preferences; next.records.append(record)
        try await saveEnqueuedCatalog(next, storage: storage, attempt: attempt)
        do {
            try await engine.enqueuePreparation(scope: scope, key: record.key, uri: client.server.mediaURL("/api/v1/downloads/\(prepared.id)/file").absoluteString,
                                                kind: item.kind == .music ? "audio" : item.kind.rawValue, wifiOnly: preferences.wifiOnly, quota: quota)
        } catch {
            try check(attempt)
            if let index = catalog.records.firstIndex(where: { $0.key == record.key }) { catalog.records[index].error = AppSession.message(error); try await storage.save(catalog) }
            throw error
        }
        await refresh()
    }

    func enqueueEpisodes(_ episodes: [MediaItem], quality: DownloadQuality, client: ServerClient) async throws {
        try await enqueueEpisodes(episodes, quality: quality, client: client, series: false)
    }

    func enqueueSeries(_ episodes: [MediaItem], quality: DownloadQuality, client: ServerClient) async throws {
        try await enqueueEpisodes(episodes, quality: quality, client: client, series: true)
    }

    private func enqueueEpisodes(_ episodes: [MediaItem], quality: DownloadQuality, client: ServerClient, series: Bool) async throws {
        guard !episodes.isEmpty, episodes.count <= 50, Set(episodes.map(\.id)).count == episodes.count,
              let first = episodes.first, !first.showID.isEmpty,
              episodes.allSatisfy({ $0.kind == .video && $0.showID == first.showID && (series || $0.season == first.season) }),
              quality != .audio else { throw ClientError.invalidInput(series ? "Choose up to 50 unique video episodes from one series." : "Choose up to 50 episodes from one season, with enough room in Downloads.") }
        guard !operation, let scope, let storage else { throw ClientError.invalidInput("Wait for the current download operation to finish.") }
        let attempt = generation
        try await performEnqueue {
            let preferences = try await enqueuePreferences(client: client, scope: scope, attempt: attempt)
            var queued = 0
            do {
                for item in episodes {
                    try check(attempt)
                    let tracks: DownloadTrackSelection?
                    if quality == .original { tracks = nil }
                    else { tracks = try await DownloadTrackSelection(audio: client.downloadTracks(itemID: item.id).audio.map(\.index), subtitles: []) }
                    try check(attempt)
                    let expected = try OfflineRecord(item: item, jobID: String(repeating: "0", count: 16), quality: quality, tracks: tracks)
                    if catalog.records.contains(where: { $0.key == expected.key }) { queued += 1; continue }
                    try await enqueueOwned(item: item, quality: quality, tracks: tracks, client: client, scope: scope, storage: storage, preferences: preferences, attempt: attempt)
                    queued += 1
                }
            } catch {
                try check(attempt)
                throw ClientError.invalidInput("Added \(queued) of \(episodes.count) episodes to Downloads. Add the \(series ? "series" : "season") again to continue. \(AppSession.message(error))")
            }
        }
    }

}
#endif
