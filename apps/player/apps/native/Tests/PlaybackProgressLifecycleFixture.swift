#if os(tvOS)
import Foundation
import Testing
@testable import KinosailPlayer

@MainActor final class ProgressSafetySource {
    let fixture: PlaybackStartupFixture
    let store: ProgressSyncStore
    let folder: URL
    let item: MediaItem
    var jobs: [ProgressSafetyOperation] = []

    init(id: String) async throws {
        let origin = try await PlaybackStartupFixture()
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("fictional-progress-" + UUID().uuidString)
        do {
            origin.denied = false
            origin.media = try PlaybackEndpointMedia.data(.source)
            origin.playbackResponse = Data("""
            {"media":{"duration":12},"plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"duration":12,"start":6,"directAllowed":true,"direct":"/media/\(id)","directType":"video/mp4"}
            """.utf8)
            let journal = try await ProgressSyncStore(scope: origin.client.profileScope(), directory: directory)
            var movie = try origin.item(id)
            var prior = WatchProgress(); prior.seconds = 6; prior.session = UUID().uuidString; prior.revision = 1
            let saved = try await origin.client.syncProgress(itemID: movie.id, progress: prior,
                                                             expected: movie.progress, playbackToken: "")
            movie.progress = saved.progress
            try #require(!saved.conflict && origin.savedSeconds == 6)
            fixture = origin; store = journal; folder = directory; item = movie
        } catch {
            await origin.client.close(purgeCache: true)
            try await origin.progressSafetyCloseAndJoin()
            try? FileManager.default.removeItem(at: directory)
            throw error
        }
    }

    func start(_ engine: PlaybackEngine) -> ProgressSafetyOperation {
        begin { try await engine.play(self.item, client: self.fixture.client, store: self.store) }
    }
    func begin(_ body: @escaping @MainActor () async throws -> Void) -> ProgressSafetyOperation {
        let operation = ProgressSafetyOperation(body); jobs.append(operation); return operation
    }
    func waitForMedia() async throws {
        let deadline = ContinuousClock.now.advanced(by: .seconds(6))
        while fixture.heldMediaCount == 0 && ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
        try #require(fixture.heldMediaCount > 0, "Actual origin media must reach the held-response barrier")
    }
    func expectPreserved(at expected: Double, label: String, allowEarlier: Bool = false) async throws {
        let pending = try await store.pending().filter { $0.itemID == item.id }
        let values = fixture.progressReceipts.dropFirst().map(\.seconds) + pending.map { $0.progress.seconds }
        #expect(abs((fixture.savedSeconds ?? -1) - expected) < 0.002)
        #expect(values.allSatisfy { allowEarlier ? $0 >= 6 && $0 <= expected + 0.002 : abs($0 - expected) < 0.002 })
        #expect(fixture.progressReceipts.allSatisfy { !$0.watched } && pending.allSatisfy { !$0.progress.watched })
        #expect(fixture.progressSafetyDroppedHTTPReceipts == 0)
        let receipt = ProgressLifecycleReceipt(caseName: label, expected: expected, server: fixture.savedSeconds,
            writes: fixture.progressReceipts.map(\.seconds), pending: pending.map { $0.progress.seconds })
        print("PLAYBACK_PROGRESS_SAFETY_LIFECYCLE " + String(decoding: try JSONEncoder().encode(receipt), as: UTF8.self))
    }
}

@MainActor final class ProgressSafetyOperation {
    var result: Result<Void, Error>?
    var completedAt: ContinuousClock.Instant?
    var task: Task<Void, Never>?
    let deadline = ContinuousClock.now.advanced(by: .seconds(12))
    init(_ body: @escaping @MainActor () async throws -> Void) {
        task = Task { @MainActor in
            do { try await body(); self.result = .success(()) }
            catch { self.result = .failure(error) }
            self.completedAt = .now
        }
    }
    func join() async throws -> Result<Void, Error> {
        while result == nil && ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
        let outcome = try #require(result, "Native operation must complete within12s")
        try #require(completedAt.map { $0 <= deadline } == true, "Native operation exceeded its12s bound")
        await task?.value; return outcome
    }
}

private struct ProgressLifecycleReceipt: Encodable {
    let caseName: String; let expected: Double; let server: Double?; let writes: [Double]; let pending: [Double]
}
#endif
