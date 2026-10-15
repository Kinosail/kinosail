#if os(tvOS)
import AVFoundation
import Foundation
import Testing
@testable import KinosailPlayer

@Suite(.serialized) @MainActor
struct PlaybackProgressSafetyJourneys {
    @Test(arguments: ["invalid-media", "media-denied", "healthy-control"])
    func failedFirstLoadCannotReplaceSavedPosition(mode: String) async throws {
        let fixture = try await PlaybackStartupFixture()
        defer { fixture.close() }
        do {
            try await exercise(mode, fixture: fixture)
            await fixture.client.close(purgeCache: true)
            try await fixture.progressSafetyCloseAndJoin()
        } catch {
            await fixture.client.close(purgeCache: true)
            try await fixture.progressSafetyCloseAndJoin()
            throw error
        }
    }

    private func exercise(_ mode: String, fixture: PlaybackStartupFixture) async throws {
        fixture.denied = false
        if mode == "invalid-media" { fixture.media = Data("fictional invalid MP4 bytes\n".utf8) }
        else { fixture.media = try PlaybackEndpointMedia.data(.source) }
        fixture.progressSafetyMediaStatus = mode == "media-denied" ? 403 : nil
        fixture.playbackResponse = Data("""
        {"media":{"duration":12},"plan":{"allowed":true,"mode":"direct","reason":"direct-preferred"},"duration":12,"start":6,"directAllowed":true,"direct":"/media/movie","directType":"video/mp4"}
        """.utf8)
        let folder = FileManager.default.temporaryDirectory
            .appendingPathComponent("fictional-progress-safety-" + UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: folder) }
        let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
        var item = try fixture.item("movie")
        var prior = WatchProgress()
        prior.seconds = 6; prior.session = UUID().uuidString; prior.revision = 1
        let accepted = try await fixture.client.syncProgress(itemID: item.id, progress: prior,
                                                             expected: item.progress, playbackToken: "")
        item.progress = accepted.progress
        try #require(fixture.savedSeconds == 6 && !accepted.conflict,
                     "Prerequisite: prior positive progress was accepted over real loopback HTTP")
        let seededReceipts = fixture.progressReceipts.count
        let engine = PlaybackEngine()
        do {
            let outcome = try await startup(engine, item: item, fixture: fixture, store: store)
            let failure: Error?
            switch outcome {
            case .success: failure = nil
            case .failure(let error): failure = error
            }
            let transportFailure = await engine.transport.failure()
            let observed = PlaybackProgressStartupObservation(
                mode: mode, engineSession: engine.generation.uuidString,
                sourceStart: engine.source?.start,
                mappedStart: engine.source.map { engine.timeline?.presentationTime($0.start) ?? $0.start },
                displayedSourceSeconds: engine.seconds, nativeStatus: engine.player?.currentItem?.status.rawValue,
                error: PlaybackSafeFailure(failure), transportError: PlaybackSafeFailure(transportFailure),
                requests: fixture.progressSafetyHTTPReceipts, droppedRequests: fixture.progressSafetyDroppedHTTPReceipts)
            let observation = try JSONEncoder().encode(observed)
            try #require(observation.count <= 16 * 1024)
            print("PLAYBACK_PROGRESS_SAFETY_STARTUP_OBSERVATION " + String(decoding: observation, as: UTF8.self))
            if mode == "healthy-control" {
                try outcome.get()
                try await drain(engine.writing, client: fixture.client)
                try await decodedProgress(try #require(engine.player))
                let deadline = ContinuousClock.now.advanced(by: .seconds(3))
                while engine.seconds <= 6 && ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
                try #require(engine.seconds > 6, "Healthy source position must advance before stop")
            } else {
                try #require(failure != nil && !(failure is CancellationError),
                             "Prerequisite: real native media load must fail without cancelling the start")
                try #require(engine.source?.start == 6 && engine.currentItem?.id == item.id)
                try #require(engine.player?.currentItem?.status == .failed,
                             "Prerequisite: this journey covers load failure, not an injected seek result")
                if mode == "media-denied" {
                    try #require((transportFailure as? ClientError) == .http(403),
                                 "Prerequisite: production gateway retained the fixture's actual HTTP403")
                }
            }
            let sourceBeforeStop = engine.seconds
            let nativeStatus = engine.player?.currentItem?.status.rawValue
            let receiptsBeforeStop = fixture.progressReceipts.count
            let monitor = engine.monitoring
            let retiredTransport = engine.transport
            engine.stop()
            await monitor?.value
            await retiredTransport.close()
            try await drain(engine.writing, client: fixture.client)
            let pending = try await store.pending().filter { $0.itemID == item.id }
            let writes = Array(fixture.progressReceipts.dropFirst(seededReceipts))
            let stopWrites = Array(fixture.progressReceipts.dropFirst(receiptsBeforeStop))
            try #require(fixture.progressSafetyDroppedHTTPReceipts == 0,
                         "A truncated fixture observation cannot qualify this regression")
            let receipt = PlaybackProgressSafetyReceipt(
                mode: mode, sourceBeforeStop: sourceBeforeStop, nativeStatus: nativeStatus,
                error: PlaybackSafeFailure(failure), transportError: PlaybackSafeFailure(transportFailure),
                finalServerSeconds: fixture.savedSeconds, postStopSeconds: stopWrites.map(\.seconds),
                publishedSeconds: writes.map(\.seconds), pendingSeconds: pending.map { $0.progress.seconds },
                requests: fixture.progressSafetyHTTPReceipts,
                sourcePositionPreserved: mode == "healthy-control"
                    ? (fixture.savedSeconds ?? 0) > 6
                        && abs((fixture.savedSeconds ?? 0) - sourceBeforeStop) < 0.002
                        && !stopWrites.isEmpty && stopWrites.allSatisfy { $0.seconds > 6 && !$0.watched }
                        && writes.contains { $0.seconds > 6 && !$0.watched }
                        && pending.allSatisfy { $0.progress.seconds > 6 && !$0.progress.watched }
                    : fixture.savedSeconds == 6 && writes.allSatisfy { $0.seconds == 6 && !$0.watched }
                        && pending.allSatisfy { $0.progress.seconds == 6 && !$0.progress.watched })
            let encoded = try JSONEncoder().encode(receipt)
            try #require(encoded.count <= 16 * 1024)
            print("PLAYBACK_PROGRESS_SAFETY_FAILED_START_RESULT " + String(decoding: encoded, as: UTF8.self))
            #expect(receipt.sourcePositionPreserved,
                    "An unsuccessful first load must not replace prior progress; a successful stop must still save advancement")
            #expect(engine.player == nil && engine.currentItem == nil)
        } catch {
            let monitor = engine.monitoring
            let retiredTransport = engine.transport
            engine.stop()
            await monitor?.value
            await retiredTransport.close()
            await fixture.client.close(purgeCache: true)
            await engine.writing?.value
            throw error
        }
    }

    @Test func safeProjectionDoesNotSerializeArbitraryRequestText() throws {
        let id = "B2D7C66A-4734-4243-BB84-6889F04631B7"
        #expect(PlaybackFailureHTTPReceipt.requestUUID(id) == id)
        for raw in ["", "Bearer fictional-sensitive-value", "https://fictional.invalid/?token=private", String(repeating: "a", count: 1000)] {
            #expect(PlaybackFailureHTTPReceipt.requestUUID(raw) == nil)
        }
        let privateMessage = "fictional-sensitive-message"
        let projected = PlaybackSafeFailure(ClientError.invalidInput(privateMessage))
        let encoded = String(decoding: try JSONEncoder().encode(projected), as: UTF8.self)
        #expect(!encoded.contains(privateMessage))
        #expect(projected.kind == .invalidInput)
    }

    private func startup(_ engine: PlaybackEngine, item: MediaItem,
                         fixture: PlaybackStartupFixture, store: ProgressSyncStore) async throws -> Result<Void, Error> {
        var outcome: Result<Void, Error>?
        let task = Task { @MainActor in
            do { try await engine.play(item, client: fixture.client, store: store); outcome = .success(()) }
            catch { outcome = .failure(error) }
        }
        do {
            let deadline = ContinuousClock.now.advanced(by: .seconds(12))
            while outcome == nil && ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
            let result = try #require(outcome, "Prerequisite: native startup finishes inside the 12s bound")
            await task.value
            return result
        } catch {
            task.cancel(); engine.stop()
            await fixture.client.close(purgeCache: true)
            await task.value
            throw error
        }
    }

    func drain(_ writer: Task<Void, Never>?, client: ServerClient) async throws {
        guard let writer else { return }
        var joined = false
        let join = Task { @MainActor in await writer.value; joined = true }
        do {
            let deadline = ContinuousClock.now.advanced(by: .seconds(3))
            while !joined && ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
            try #require(joined, "Fixture progress update must finish within3s")
            await join.value
        } catch {
            await client.close(purgeCache: true)
            await join.value
            throw error
        }
    }

    func decodedProgress(_ player: AVPlayer, output supplied: AVPlayerItemVideoOutput? = nil) async throws {
        let item = try #require(player.currentItem)
        try #require(abs(item.duration.seconds - 12) < 0.002)
        let output = supplied ?? AVPlayerItemVideoOutput(pixelBufferAttributes: [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA])
        if supplied == nil { item.add(output) }
        defer { if supplied == nil { item.remove(output) } }
        try #require(item.outputs.contains { $0 === output })
        let initial = player.currentTime().seconds
        let deadline = ContinuousClock.now.advanced(by: .seconds(3))
        var decoded = false
        var requested: Double?, displayed: Double?, acquired = 0
        while ContinuousClock.now < deadline {
            let time = player.currentTime()
            if time.seconds > initial + 0.3 {
                requested = time.seconds
                var display = CMTime.invalid
                if output.copyPixelBuffer(forItemTime: time, itemTimeForDisplay: &display) != nil {
                    acquired += 1
                    displayed = display.seconds.isFinite ? display.seconds : nil
                    if let displayed, displayed > initial { decoded = true; break }
                }
            }
            try await Task.sleep(for: .milliseconds(10))
        }
        let end = player.currentTime().seconds
        let observation = ProgressDecodeObservation(initial: initial.isFinite ? initial : nil,
            final: end.isFinite ? end : nil, rate: player.rate, nativeStatus: item.status.rawValue,
            timeControlStatus: player.timeControlStatus.rawValue, decoded: decoded,
            earlyOutput: supplied != nil, outputAttached: item.outputs.contains { $0 === output },
            requested: requested, displayed: displayed, acquired: acquired,
            failure: PlaybackSafeFailure(item.error))
        print("PLAYBACK_PROGRESS_SAFETY_DECODE " + String(decoding: try JSONEncoder().encode(observation), as: UTF8.self))
        try #require(decoded, "Healthy control requires real decoded advancement, not state alone")
    }
}

private struct ProgressDecodeObservation: Encodable {
    let initial: Double?; let final: Double?; let rate: Float; let nativeStatus: Int
    let timeControlStatus: Int; let decoded: Bool; let earlyOutput: Bool; let outputAttached: Bool
    let requested: Double?; let displayed: Double?; let acquired: Int; let failure: PlaybackSafeFailure
}

private struct PlaybackProgressSafetyReceipt: Encodable {
    let mode: String
    let sourceBeforeStop: Double
    let nativeStatus: Int?
    let error: PlaybackSafeFailure
    let transportError: PlaybackSafeFailure
    let finalServerSeconds: Double?
    let postStopSeconds: [Double]
    let publishedSeconds: [Double]
    let pendingSeconds: [Double]
    let requests: [PlaybackFailureHTTPReceipt]
    let sourcePositionPreserved: Bool
}

private struct PlaybackProgressStartupObservation: Encodable {
    let mode: String
    let engineSession: String
    let sourceStart: Double?
    let mappedStart: Double?
    let displayedSourceSeconds: Double
    let nativeStatus: Int?
    let error: PlaybackSafeFailure
    let transportError: PlaybackSafeFailure
    let requests: [PlaybackFailureHTTPReceipt]
    let droppedRequests: Int
}
#endif
