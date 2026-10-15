import Foundation
import Testing
@testable import KinosailPlayer

@Suite(.serialized, .enabled(if: FileManager.default.fileExists(atPath: "/tmp/kinosail-player-buffer-bar.mp4")))
@MainActor struct ProgressSyncRecoveryJourneys {
    private func progress(_ seconds: Double, _ session: String, _ revision: Int) -> WatchProgress {
        var value = WatchProgress(); value.seconds = seconds; value.session = session; value.revision = revision; return value
    }

    private func environment() async throws -> (PlaybackStartupFixture, ProgressSyncStore, URL) {
        let fixture = try await PlaybackStartupFixture()
        fixture.denied = false
        fixture.media = try Data(contentsOf: URL(fileURLWithPath: "/tmp/kinosail-player-buffer-bar.mp4"))
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
        return (fixture, store, folder)
    }

    private func settle(_ store: ProgressSyncStore, itemID: String, fixture: PlaybackStartupFixture, conflictsBefore: Int = 0) async throws -> [PendingProgress] {
        let deadline = ContinuousClock.now.advanced(by: .seconds(5))
        while ContinuousClock.now < deadline, fixture.conflicts == conflictsBefore, !fixture.progressReceipts.contains(where: { $0.itemID == itemID }) {
            try await Task.sleep(for: .milliseconds(20))
        }
        while ContinuousClock.now < deadline {
            let pending = try await store.pending().filter { $0.itemID == itemID }
            if pending.isEmpty { return pending }
            try await Task.sleep(for: .milliseconds(20))
        }
        return try await store.pending().filter { $0.itemID == itemID }
    }

    @Test func removedTitleIsDroppedInsteadOfRetriedForever() async throws {
        let (fixture, store, folder) = try await environment()
        defer { fixture.close(); try? FileManager.default.removeItem(at: folder) }
        fixture.serverProgress = ["movie": WatchProgress()]
        fixture.missingItems = ["gone"]
        try await store.record(itemID: "gone", progress: progress(5, UUID().uuidString, 1), expected: WatchProgress())
        try await store.record(itemID: "movie", progress: progress(7, UUID().uuidString, 1), expected: WatchProgress())
        _ = try await store.synchronize(client: fixture.client)
        _ = try await store.synchronize(client: fixture.client)
        let pending = try await store.pending().map(\.itemID)
        let goneRequests = fixture.count("/api/v1/items/gone/progress/sync")
        print("PROGRESS_RECOVERY gone pending=\(pending) goneRequests=\(goneRequests) movieSaved=\(fixture.savedSeconds ?? -1)")
        #expect(pending.isEmpty)
        #expect(goneRequests == 1)
        #expect(fixture.savedSeconds == 7)
        await fixture.client.close(purgeCache: true)
    }

    @Test func playbackFromStaleCachedTitleStillSavesProgress() async throws {
        let (fixture, store, folder) = try await environment()
        defer { fixture.close(); try? FileManager.default.removeItem(at: folder) }
        fixture.serverProgress = ["movie": progress(6, "other-device", 3)]
        let stale = try fixture.item("movie")
        let coordinator = PlaybackCoordinator()
        defer { coordinator.stop() }
        try await coordinator.play(stale, client: fixture.client, store: store)
        coordinator.checkpoint()
        let pending = try await settle(store, itemID: "movie", fixture: fixture)
        let saved = fixture.serverProgress?["movie"]
        print("PROGRESS_RECOVERY stale conflicts=\(fixture.conflicts) pending=\(pending.count) serverSession=\(saved?.session ?? "nil")")
        #expect(fixture.conflicts == 0)
        #expect(pending.isEmpty)
        #expect(saved?.session != "other-device")
        await fixture.client.close(purgeCache: true)
    }

    @Test func playingTitleAgainReplacesItsUnresolvedConflict() async throws {
        let (fixture, store, folder) = try await environment()
        defer { fixture.close(); try? FileManager.default.removeItem(at: folder) }
        fixture.serverProgress = ["movie": progress(9, "other-device", 9)]
        try await store.record(itemID: "movie", progress: progress(4, "earlier-session", 2), expected: progress(2, "older", 1))
        _ = try await store.synchronize(client: fixture.client)
        try #require(try await store.pending().first?.conflict != nil, "Prerequisite: an earlier session left a conflict")
        let coordinator = PlaybackCoordinator()
        defer { coordinator.stop() }
        try await coordinator.play(try fixture.item("movie"), client: fixture.client, store: store)
        coordinator.checkpoint()
        let pending = try await settle(store, itemID: "movie", fixture: fixture, conflictsBefore: 1)
        let saved = fixture.serverProgress?["movie"]
        print("PROGRESS_RECOVERY conflict pending=\(pending.count) conflicts=\(fixture.conflicts) serverSession=\(saved?.session ?? "nil")")
        #expect(pending.isEmpty)
        #expect(fixture.conflicts == 1)
        #expect(saved?.session != "other-device" && saved?.session != "earlier-session")
        await fixture.client.close(purgeCache: true)
    }
}
