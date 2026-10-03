import AVFoundation
import Foundation
import Synchronization
import Testing
@testable import KinosailPlayer

// Failure paths were enumerated before implementation. These loopback journeys
// retain deterministic cancellation ordering unavailable through a UI tap.
@Suite(.serialized) @MainActor
struct PlaybackStartupJourneys {
    private func wait(_ condition: () -> Bool) async throws {
        for _ in 0..<500 {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(10))
        }
        Issue.record("Fixture did not reach its expected request barrier")
        throw CancellationError()
    }
    @Test func cancelledStartPreservesActivePlaybackWithoutRequests() async throws {
        let fixture = try await PlaybackStartupFixture()
        defer { fixture.close() }
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: folder) }
        let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
        let engine = PlaybackEngine()
        engine.currentItem = try fixture.item("active")
        let original = AVPlayer()
        engine.player = original
        let generation = engine.generation
        defer { engine.stop() }
        let task = Task { try await engine.play(fixture.item("cancelled"), client: fixture.client, store: store) }
        task.cancel()
        _ = await task.result
        #expect(engine.currentItem?.id == "active")
        #expect(engine.player === original)
        #expect(engine.generation == generation)
        #expect(fixture.count("/api/v1/items/cancelled/playback") == 0)
        print("STARTUP cancelled-start preserved=\(engine.player === original) reads=\(fixture.count("/api/v1/items/cancelled/playback"))")
        await fixture.client.close(purgeCache: true)
    }
    @Test func terminalDenialDoesNotRepeatNegotiation() async throws {
        let fixture = try await PlaybackStartupFixture()
        defer { fixture.close() }
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: folder) }
        let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
        let engine = PlaybackEngine()
        defer { engine.stop() }
        await #expect(throws: ClientError.http(403)) { try await engine.play(fixture.item("denied"), client: fixture.client, store: store) }
        #expect(fixture.count("/api/v1/items/denied/playback") == 1)
        #expect(engine.player == nil)
        print("STARTUP terminal-denial sourceReads=\(fixture.count("/api/v1/items/denied/playback"))")
        await fixture.client.close(purgeCache: true)
    }
    @Test func cancelledPreparationReturnsPromptly() async throws {
        let fixture = try await PlaybackStartupFixture()
        fixture.delayed = true
        defer { fixture.close() }
        let engine = PlaybackEngine()
        let finished = Mutex<ContinuousClock.Instant?>(nil)
        let task = Task {
            defer { finished.withLock { $0 = .now } }
            try await engine.prepare(fixture.item("a"), client: fixture.client)
        }
        try await wait { fixture.count("/api/v1/items/a/playback") == 1 }
        let start = ContinuousClock.now
        task.cancel()
        try await Task.sleep(for: .milliseconds(100))
        #expect(finished.withLock { $0 != nil })
        _ = await task.result
        let elapsed = start.duration(to: finished.withLock { $0 } ?? .now)
        print("STARTUP cancelled-preparation elapsed=\(elapsed)")
        await fixture.client.close(purgeCache: true)
        engine.stop()
    }
    @Test func backgroundPreparationCannotSupersedeActiveStartup() async throws {
        let fixture = try await PlaybackStartupFixture()
        fixture.delayed = true; fixture.denied = false
        defer { fixture.close() }
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: folder) }
        let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
        let engine = PlaybackEngine()
        let foreground = Task { try await engine.play(fixture.item("a"), client: fixture.client, store: store) }
        try await wait { fixture.count("/api/v1/items/a/playback") == 1 }
        try? await engine.prepare(fixture.item("b"), client: fixture.client)
        #expect(fixture.count("/api/v1/items/b/playback") == 0)
        foreground.cancel()
        await fixture.client.close(purgeCache: true)
        _ = await foreground.result
        engine.stop()
    }
    @Test func latePreparationCannotClearNewerSameTitle() async throws {
        let fixture = try await PlaybackStartupFixture()
        fixture.delayed = true; fixture.denied = false
        defer { fixture.close() }
        let engine = PlaybackEngine()
        let a1 = Task { try await engine.prepare(fixture.item("a"), client: fixture.client) }
        try await wait { fixture.count("/api/v1/items/a/playback") == 1 && fixture.count("/api/v1/items/a/playback-preferences") == 1 }
        let b = Task { try await engine.prepare(fixture.item("b"), client: fixture.client) }
        try await wait { fixture.count("/api/v1/items/b/playback") == 1 }
        let a2 = Task { try await engine.prepare(fixture.item("a"), client: fixture.client) }
        try await wait { fixture.count("/api/v1/items/a/playback") == 2 }
        _ = await a1.result
        let joinA2 = Task { try await engine.prepare(fixture.item("a"), client: fixture.client) }
        _ = await b.result; _ = await a2.result
        try await joinA2.value
        #expect(fixture.count("/api/v1/items/a/playback") == 2)
        print("STARTUP A-B-A sourceReads=\(fixture.count("/api/v1/items/a/playback"))")
        await fixture.client.close(purgeCache: true)
        engine.stop()
    }
}
