#if os(tvOS)
import AVFoundation
import Foundation
import Testing
@testable import KinosailPlayer

// Isolation gap: production profiles cannot be reseeded or have real media
// responses held to deterministically exercise interrupted native loading.
extension PlaybackProgressSafetyJourneys {
    @Test func pendingInitialSeekCannotPublishUncommittedProgress() async throws {
        try await withSources(count: 1) { engine, sources in
            let source = sources[0]
            source.fixture.progressSafetyMediaStatus = 403
            source.fixture.holdMedia = true
            let start = source.start(engine)
            try await source.waitForMedia()
            try #require(engine.loading && engine.writer != nil)
            try await engine.seek(to: 9)
            engine.checkpoint(); try await drain(engine.writing, client: source.fixture.client)
            engine.pause(); try await drain(engine.writing, client: source.fixture.client)
            source.fixture.releaseMedia()
            let result = try await start.join()
            #expect(PlaybackSafeFailure(result.error).kind == .http)
            engine.stop(); try await drain(engine.writing, client: source.fixture.client)
            try await source.expectPreserved(at: 6, label: "pending-initial-seek")
        }
    }

    @Test func failedReplacementSavesLastCommittedPosition() async throws {
        try await withSources(count: 1) { engine, sources in
            let source = sources[0]
            try (await source.start(engine).join()).get()
            try await drain(engine.writing, client: source.fixture.client)
            engine.resume()
            try await decodedProgress(try #require(engine.player))
            engine.pause(); try await drain(engine.writing, client: source.fixture.client)
            let committed = engine.seconds
            try #require(committed > 6 && committed < 9)
            // The production retry entry owns replacement. Stop its independent
            // polling task to make one recovery request the sole stimulus.
            let monitor = engine.monitoring; monitor?.cancel(); await monitor?.value
            let details = try #require(engine.source)
            let attempt = engine.generation
            source.fixture.progressSafetyMediaStatus = 403
            source.fixture.holdMedia = true
            let replacement = source.begin {
                try await engine.install(details: details, compatible: false, at: committed,
                                         attempt: attempt, failure: URLError(.networkConnectionLost))
            }
            try await source.waitForMedia()
            try #require(engine.recoveringNetwork)
            try await engine.seek(to: 9)
            engine.checkpoint(); try await drain(engine.writing, client: source.fixture.client)
            engine.pause(); try await drain(engine.writing, client: source.fixture.client)
            source.fixture.releaseMedia()
            let result = try await replacement.join()
            #expect(PlaybackSafeFailure(result.error).kind == .http)
            engine.stop(); try await drain(engine.writing, client: source.fixture.client)
            try await source.expectPreserved(at: committed, label: "failed-replacement", allowEarlier: true)
        }
    }

    @Test func failedNextTitleCannotInheritPriorTitleCommitment() async throws {
        try await withSources(count: 2) { engine, sources in
            let first = sources[0], second = sources[1]
            try (await first.start(engine).join()).get()
            try await drain(engine.writing, client: first.fixture.client)
            engine.resume()
            try await decodedProgress(try #require(engine.player))
            engine.pause(); try await drain(engine.writing, client: first.fixture.client)
            let committed = engine.seconds
            try #require(committed > 6)
            second.fixture.progressSafetyMediaStatus = 403
            second.fixture.holdMedia = true
            let next = second.start(engine)
            try await second.waitForMedia()
            try await drain(engine.writing, client: first.fixture.client)
            second.fixture.releaseMedia()
            let result = try await next.join()
            #expect(PlaybackSafeFailure(result.error).kind == .http)
            engine.stop(); try await drain(engine.writing, client: second.fixture.client)
            try await second.expectPreserved(at: 6, label: "failed-next-title")
            try await first.expectPreserved(at: committed, label: "retired-healthy-title", allowEarlier: true)
        }
    }

    @Test func staleInitialLoadCannotPublishIntoReplacementTitle() async throws {
        try await withSources(count: 2) { engine, sources in
            let first = sources[0], second = sources[1]
            first.fixture.progressSafetyMediaStatus = 403
            first.fixture.holdMedia = true
            let stale = first.start(engine)
            try await first.waitForMedia()
            let oldGeneration = engine.generation
            second.fixture.holdMedia = true
            let healthy = second.start(engine)
            let deadline = ContinuousClock.now.advanced(by: .seconds(6))
            while (engine.generation == oldGeneration || engine.client !== second.fixture.client), ContinuousClock.now < deadline {
                try await Task.sleep(for: .milliseconds(10))
            }
            try #require(engine.generation != oldGeneration && engine.client === second.fixture.client,
                         "Replacement must own the engine before the stale origin responds")
            try await second.waitForMedia()
            try await drain(engine.writing, client: first.fixture.client)
            first.fixture.releaseMedia()
            second.fixture.releaseMedia()
            let oldResult = try await stale.join()
            #expect(oldResult.error is CancellationError)
            try (await healthy.join()).get()
            try await drain(engine.writing, client: second.fixture.client)
            engine.resume()
            try await decodedProgress(try #require(engine.player))
            engine.pause(); try await drain(engine.writing, client: second.fixture.client)
            let committed = engine.seconds
            try #require(committed > 6)
            engine.stop(); try await drain(engine.writing, client: second.fixture.client)
            try await first.expectPreserved(at: 6, label: "stale-initial-load")
            try await second.expectPreserved(at: committed, label: "replacement-healthy-title", allowEarlier: true)
        }
    }

    private func withSources(count: Int, body: (PlaybackEngine, [ProgressSafetySource]) async throws -> Void) async throws {
        let engine = PlaybackEngine()
        var sources: [ProgressSafetySource] = []
        do {
            for index in 0..<count { sources.append(try await ProgressSafetySource(id: "movie-\(index)")) }
            try await body(engine, sources)
        } catch {
            await clean(engine, sources: sources)
            throw error
        }
        await clean(engine, sources: sources)
    }

    private func clean(_ engine: PlaybackEngine, sources: [ProgressSafetySource]) async {
        sources.forEach { $0.jobs.forEach { $0.task?.cancel() } }
        let monitor = engine.monitoring, transport = engine.transport
        engine.stop()
        // Close fictional origins before joining cancellation paths; a held
        // response must not keep a failed test's writer or startup alive.
        for source in sources {
            source.fixture.releaseMedia()
            await source.fixture.client.close(purgeCache: true)
        }
        await monitor?.value; await transport.close(); await engine.writing?.value
        for source in sources {
            for job in source.jobs { await job.task?.value }
            do { try await source.fixture.progressSafetyCloseAndJoin() }
            catch { Issue.record("Fictional fixture cleanup failed") }
            try? FileManager.default.removeItem(at: source.folder)
        }
    }
}

@MainActor private final class ProgressSafetySource {
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

@MainActor private final class ProgressSafetyOperation {
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

private extension Result where Success == Void, Failure == Error {
    var error: Error? { if case .failure(let error) = self { return error }; return nil }
}
private struct ProgressLifecycleReceipt: Encodable {
    let caseName: String; let expected: Double; let server: Double?; let writes: [Double]; let pending: [Double]
}
#endif
