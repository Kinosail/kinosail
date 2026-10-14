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
                let installation = engine.beginInstallation()
                try await engine.install(details: details, compatible: false, at: committed,
                                         attempt: attempt, installation: installation, failure: URLError(.networkConnectionLost))
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

    // Distinct gap: replacing the stream for the same title retains generation.
    // A retired load must not clear pending state, seek the new player, or resume
    // it after the user's pause. Hold real media after player creation; this
    // does not claim to hold the operating system's audio activation call.
    @Test func supersededSameTitleLoadPreservesLatestSeekAndPause() async throws {
        try await withSources(count: 1) { engine, sources in
            let source = sources[0]
            try (await source.start(engine).join()).get()
            engine.pause(); try await drain(engine.writing, client: source.fixture.client)
            let monitor = engine.monitoring; monitor?.cancel(); await monitor?.value
            let details = try #require(engine.source), attempt = engine.generation
            source.fixture.holdMedia = true
            let retired = source.begin {
                let installation = engine.beginInstallation()
                try await engine.install(details: details, compatible: false, at: 6,
                                         attempt: attempt, installation: installation)
            }
            try await source.waitForMedia()
            let oldPlayer = try #require(engine.player)
            let heldBefore = source.fixture.heldMediaCount
            let replacement = source.begin {
                let installation = engine.beginInstallation()
                try await engine.install(details: details, compatible: false, at: 6,
                                         attempt: attempt, installation: installation)
            }
            let deadline = ContinuousClock.now.advanced(by: .seconds(6))
            while (engine.player == nil || engine.player === oldPlayer || source.fixture.heldMediaCount <= heldBefore),
                  ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
            let newPlayer = try #require(engine.player)
            try #require(newPlayer !== oldPlayer && source.fixture.heldMediaCount > heldBefore)
            // Observe this item's decoder before releasing preparation. The
            // retained late-attachment trial advanced without yielding a frame.
            let newItem = try #require(newPlayer.currentItem)
            let output = AVPlayerItemVideoOutput(pixelBufferAttributes: [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA])
            newItem.add(output)
            defer { newItem.remove(output) }
            try #require(engine.generation == attempt && engine.loading)
            try await engine.seek(to: 9)
            engine.pause(); try await drain(engine.writing, client: source.fixture.client)
            #expect((try await retired.join()).error is CancellationError)
            #expect(engine.loading && engine.seconds == 9 && engine.player === newPlayer)
            #expect(source.fixture.progressReceipts.allSatisfy { !$0.watched && $0.seconds < 9 })
            source.fixture.releaseMedia()
            try (await replacement.join()).get()
            #expect(!engine.loading && engine.player === newPlayer)
            #expect(abs(newPlayer.currentTime().seconds - 9) < 0.1)
            #expect(newPlayer.timeControlStatus == .paused && newPlayer.rate == 0)
            try #require(newPlayer.currentItem === newItem && newItem.outputs.contains { $0 === output })
            engine.resume(); try await decodedProgress(newPlayer, output: output)
            engine.pause(); try await drain(engine.writing, client: source.fixture.client)
            #expect(engine.seconds > 9 && engine.seconds < 12 && !engine.completed)
            print("PLAYBACK_INSTALLATION sameTitle=true retiredCancelled=true latestSeek=9 paused=true decodedReplacement=true joined=true")
        }
    }

    // The retained late-output control fails without a retired installation.
    // Match early observation while preserving the seek and frame requirements.
    @Test func healthyPendingSeekToNineSupportsEarlyFrameObservation() async throws {
        try await withSources(count: 1) { engine, sources in
            let source = sources[0]
            source.fixture.holdMedia = true
            let startup = source.start(engine)
            try await source.waitForMedia()
            let player = try #require(engine.player)
            let item = try #require(player.currentItem)
            let output = AVPlayerItemVideoOutput(pixelBufferAttributes: [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA])
            item.add(output)
            defer { item.remove(output) }
            try #require(engine.loading)
            try await engine.seek(to: 9)
            engine.pause(); try await drain(engine.writing, client: source.fixture.client)
            source.fixture.releaseMedia()
            try (await startup.join()).get()
            try #require(!engine.loading && engine.player === player)
            try #require(abs(player.currentTime().seconds - 9) < 0.1)
            try #require(player.timeControlStatus == .paused && player.rate == 0)
            try #require(player.currentItem === item && item.outputs.contains { $0 === output })
            engine.resume(); try await decodedProgress(player, output: output)
            engine.pause(); try await drain(engine.writing, client: source.fixture.client)
            #expect(engine.seconds > 9 && engine.seconds < 12 && !engine.completed)
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

private extension Result where Success == Void, Failure == Error {
    var error: Error? { if case .failure(let error) = self { return error }; return nil }
}
#endif
