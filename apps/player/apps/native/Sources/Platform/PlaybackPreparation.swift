import Foundation

struct PlaybackPreparation: Sendable {
    let itemID: String
    let clientID: UUID
    let saved: Date
    let source: PlaybackSource
    let preferences: PlaybackPreferences
}

/// The producer owns its result; cancelling a screen only removes that waiter.
/// Foreground playback can adopt the same request when title details disappear.
@MainActor
final class PlaybackPreparationRequest {
    let itemID: String
    let clientID: UUID
    var task: Task<Void, Never>?
    private var result: Result<PlaybackPreparation, Error>?
    private var waiters: [UUID: CheckedContinuation<PlaybackPreparation, Error>] = [:]

    init(itemID: String, clientID: UUID) { self.itemID = itemID; self.clientID = clientID }

    func value() async throws -> PlaybackPreparation {
        let id = UUID()
        let prepared = try await withTaskCancellationHandler {
            try Task.checkCancellation()
            return try await withCheckedThrowingContinuation { continuation in
                if let result { continuation.resume(with: result) }
                else { waiters[id] = continuation }
            }
        } onCancel: { Task { @MainActor in self.waiters.removeValue(forKey: id)?.resume(throwing: CancellationError()) } }
        try Task.checkCancellation()
        return prepared
    }

    func finish(_ result: Result<PlaybackPreparation, Error>) {
        guard self.result == nil else { return }
        self.result = result
        task = nil
        let pending = Array(waiters.values)
        waiters.removeAll()
        pending.forEach { $0.resume(with: result) }
    }

    func cancel() { task?.cancel(); finish(.failure(CancellationError())) }
}

extension PlaybackEngine {
    private static let preparationLifetime: TimeInterval = 30

    func prepare(_ item: MediaItem, client: ServerClient) async throws {
        try Task.checkCancellation()
        guard !loading, [.video, .music, .audiobook].contains(item.kind) else { return }
        _ = try await preparedPlayback(for: item, client: client)
    }

    func preparedPlayback(for item: MediaItem, client: ServerClient) async throws -> PlaybackPreparation {
        try Task.checkCancellation()
        let clientID = client.identity
        if let playbackPreparation,
           playbackPreparation.itemID == item.id,
           playbackPreparation.clientID == clientID,
           Date().timeIntervalSince(playbackPreparation.saved) < Self.preparationLifetime {
            return playbackPreparation
        }
        if let request = playbackPreparationRequest, request.itemID == item.id, request.clientID == clientID {
            return try await request.value()
        }
        playbackPreparationRequest?.cancel()
        let request = PlaybackPreparationRequest(itemID: item.id, clientID: clientID)
        playbackPreparationRequest = request
        request.task = Task { @MainActor in
            do {
                let prepared = try await fetchPlaybackPreparation(for: item, client: client)
                try Task.checkCancellation()
                if playbackPreparationRequest === request {
                    playbackPreparation = prepared
                    playbackPreparationRequest = nil
                }
                request.finish(.success(prepared))
            } catch {
                if playbackPreparationRequest === request {
                    playbackPreparation = nil
                    playbackPreparationRequest = nil
                }
                request.finish(.failure(error))
            }
        }
        return try await request.value()
    }

    func fetchPlaybackPreparation(for item: MediaItem, client: ServerClient) async throws -> PlaybackPreparation {
        async let source = client.playback(itemID: item.id)
        async let preferences = client.playbackPreparationPreferences(itemID: item.id)
        let prepared = PlaybackPreparation(itemID: item.id, clientID: client.identity, saved: Date(),
                                           source: try await source, preferences: try await preferences.playback)
        return try prepared.validated()
    }
}

private extension PlaybackPreparation {
    func validated() throws -> Self {
        if preferences.audioEnhancementsEnabled, source.direct != nil || source.compatible == nil {
            throw ClientError.invalidInput("Update Kinosail Server to use audio enhancements.")
        }
        return self
    }
}
