import Foundation

struct PlaybackPreparation: Sendable {
    let itemID: String
    let clientID: UUID
    let saved: Date
    let source: PlaybackSource
    let preferences: PlaybackPreferences
}

extension PlaybackEngine {
    private static let preparationLifetime: TimeInterval = 30

    func prepare(_ item: MediaItem, client: ServerClient) async throws {
        guard [.video, .music, .audiobook].contains(item.kind) else { return }
        _ = try await preparedPlayback(for: item, client: client)
    }

    func preparedPlayback(for item: MediaItem, client: ServerClient) async throws -> PlaybackPreparation {
        let clientID = client.identity
        if let playbackPreparation,
           playbackPreparation.itemID == item.id,
           playbackPreparation.clientID == clientID,
           Date().timeIntervalSince(playbackPreparation.saved) < Self.preparationLifetime {
            return playbackPreparation
        }
        if playbackPreparationItemID == item.id,
           playbackPreparationClientID == clientID,
           let task = playbackPreparationTask {
            return try await task.value
        }
        playbackPreparationTask?.cancel()
        let task = Task { @MainActor in
            async let source = client.playback(itemID: item.id)
            async let preferences = client.playbackPreferences(itemID: item.id)
            return PlaybackPreparation(itemID: item.id, clientID: clientID, saved: Date(),
                                       source: try await source, preferences: try await preferences.playback)
        }
        playbackPreparationTask = task
        playbackPreparationItemID = item.id
        playbackPreparationClientID = clientID
        do {
            let prepared = try await task.value
            if playbackPreparationItemID == item.id, playbackPreparationClientID == clientID {
                playbackPreparation = prepared
                playbackPreparationTask = nil
            }
            return prepared
        } catch {
            if playbackPreparationItemID == item.id, playbackPreparationClientID == clientID {
                playbackPreparation = nil
                playbackPreparationTask = nil
            }
            throw error
        }
    }

    func fetchPlaybackPreparation(for item: MediaItem, client: ServerClient) async throws -> PlaybackPreparation {
        async let source = client.playback(itemID: item.id)
        async let preferences = client.playbackPreferences(itemID: item.id)
        return PlaybackPreparation(itemID: item.id, clientID: client.identity, saved: Date(),
                                   source: try await source, preferences: try await preferences.playback)
    }
}
