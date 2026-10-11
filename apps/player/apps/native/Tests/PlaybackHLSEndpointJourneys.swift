#if os(tvOS)
import AVFoundation
import SwiftUI
import Synchronization
import Testing
@testable import KinosailPlayer

// Test-first isolation gap: deterministic sub-frame pending restoration through
// visible retained AVKit cannot be injected into a real profile's journal.
// This uses fictional loopback HLS and production NativePlayerView/coordinator.
@Suite(.serialized) struct PlaybackEndpointJourneys {}

extension PlaybackEndpointJourneys {
    @Suite
    @MainActor struct PlaybackHLSEndpointJourneys {
        @Test(arguments: [0.0, 0.024], [false, true])
        func retainedVisibleHLSFinishesAtMappedPendingEndpoint(remaining: Double, prepared: Bool) async throws {
            try await endpointProbe(remaining: remaining, prepared: prepared, visible: true)
        }

        @Test(arguments: [0.0, 0.024], [false, true])
        func retainedUnattachedHLSFinishesAtMappedPendingEndpoint(remaining: Double, prepared: Bool) async throws {
            try await endpointProbe(remaining: remaining, prepared: prepared, visible: false)
        }

        static func compatibleFixture() async throws -> PlaybackStartupFixture {
            let manifest = try PlaybackEndpointMedia.data(.manifest)
            let segment = try PlaybackEndpointMedia.data(.segment)
            try #require(manifest.count + segment.count <= 1024 * 1024)
            let fixture = try await PlaybackStartupFixture()
            fixture.denied = false
            fixture.hlsResources = ["/hls/movie/index.m3u8": manifest, "/hls/movie/segment-00.ts": segment]
            fixture.playbackResponse = Data("""
            {"media":{"duration":15},"plan":{"allowed":true,"mode":"direct","reason":"fixture-original-unsupported"},"duration":15,"start":6,"directAllowed":false,"compatibleDuration":12,"compatible":"/hls/movie/index.m3u8","compatibleProgressToken":"fictional-omitted-opening-token","compatiblePlan":{"allowed":true,"mode":"remux","reason":"compatibility-requested","markerMode":"server","timeline":{"sourceDuration":15,"duration":12,"omitted":[{"start":0,"end":3}]}}}
            """.utf8)
            return fixture
        }

        private func endpointProbe(remaining: Double, prepared: Bool, visible: Bool) async throws {
            let fixture = try await Self.compatibleFixture()
            defer { fixture.close() }
            let item = try fixture.item("movie")
            let coordinator = PlaybackCoordinator()
            defer { coordinator.onCompleted = nil; coordinator.stop() }
            let retainedController = coordinator.presentation.controller
            let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
            let previous = scene.windows.first { $0.isKeyWindow }
            let window = UIWindow(windowScene: scene)
            window.rootViewController = UIHostingController(rootView: HLSEndpointProbeSurface(coordinator: coordinator))
            window.makeKeyAndVisible()
            defer { window.isHidden = true; previous?.makeKey() }

            let warmStore = try await fictionalStore(fixture)
            _ = try await boundedPlay(coordinator, item: item, fixture: fixture, store: warmStore)
            let warmPlayer = try #require(coordinator.player)
            let warmItem = try #require(warmPlayer.currentItem)
            let output = AVPlayerItemVideoOutput(pixelBufferAttributes: [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA])
            warmItem.add(output)
            let first = warmPlayer.currentTime().seconds
            let decodeDeadline = ContinuousClock.now.advanced(by: .seconds(12))
            var decoded = false
            while ContinuousClock.now < decodeDeadline {
                if warmPlayer.currentTime().seconds > first + 0.1,
                   output.copyPixelBuffer(forItemTime: warmPlayer.currentTime(), itemTimeForDisplay: nil) != nil,
                   coordinator.presentation.readyForDisplay, retainedController.view.window === window {
                    decoded = true; break
                }
                try await Task.sleep(for: .milliseconds(10))
            }
            try #require(decoded, "Prerequisite: visible HLS interior must decode and advance before retained-controller replay")
            try #require(abs(warmItem.duration.seconds - 12) < 0.002, "Fixture native HLS duration must match declared duration")
            try #require(coordinator.usingCompatibility)
            coordinator.stop()
            try #require(coordinator.presentation.controller === retainedController)
            let detachDeadline = ContinuousClock.now.advanced(by: .seconds(2))
            while (retainedController.view.window != nil || coordinator.presentation.visible), ContinuousClock.now < detachDeadline {
                try await Task.sleep(for: .milliseconds(10))
            }
            try #require(retainedController.view.window == nil && !coordinator.presentation.visible,
                         "Prerequisite: prior native view must finish exit before retained replay")
            if !visible { window.rootViewController = UIHostingController(rootView: Color.black.ignoresSafeArea()) }

            // Warm playback caches preparation for thirty seconds. A fresh fictional
            // client makes the cold/prepared distinction real without private access.
            let endpointFixture = try await PlaybackStartupFixture()
            endpointFixture.denied = false
            endpointFixture.hlsResources = fixture.hlsResources
            endpointFixture.playbackResponse = fixture.playbackResponse
            defer { endpointFixture.close() }
            let endpointItem = try endpointFixture.item("movie")
            let endpointDirectory = FileManager.default.temporaryDirectory.appendingPathComponent("fictional-hls-endpoint-" + UUID().uuidString)
            let endpointStore = try await fictionalStore(endpointFixture, directory: endpointDirectory)
            var pending = WatchProgress()
            pending.seconds = 15 - remaining; pending.session = UUID().uuidString; pending.revision = 1
            try await endpointStore.record(itemID: endpointItem.id, progress: pending, expected: WatchProgress())
            let playbackPath = "/api/v1/items/movie/playback"
            let preferencesPath = "/api/v1/items/movie/playback-preferences"
            try #require(endpointFixture.count(playbackPath) == 0 && endpointFixture.count(preferencesPath) == 0)
            if prepared { try await coordinator.prepare(endpointItem, client: endpointFixture.client) }
            let beforePlaySource = endpointFixture.count(playbackPath)
            let beforePlayPreferences = endpointFixture.count(preferencesPath)
            try #require(beforePlaySource == (prepared ? 1 : 0) && beforePlayPreferences == (prepared ? 1 : 0),
                         "Prerequisite: cold/prepared case must have distinct preparation history")
            var completions = 0
            var committedAtEnd: Double?
            coordinator.onCompleted = { completedItem, _ in
                if completedItem.id == endpointItem.id {
                    if completions == 0 { committedAtEnd = coordinator.seconds }
                    completions += 1
                }
            }
            if !visible {
                try #require(retainedController.player == nil && retainedController.view.window == nil && !coordinator.presentation.visible)
            }
            let observedEnds = Mutex<[(ObjectIdentifier, ContinuousClock.Instant)]>([])
            let nativeEndObserver = NotificationCenter.default.addObserver(forName: .AVPlayerItemDidPlayToEndTime, object: nil, queue: nil) { notification in
                guard let ended = notification.object as? AVPlayerItem else { return }
                observedEnds.withLock { $0.append((ObjectIdentifier(ended), .now)) }
            }
            defer { NotificationCenter.default.removeObserver(nativeEndObserver) }
            let start = ContinuousClock.now
            let startupReturned = try await boundedPlay(coordinator, item: endpointItem, fixture: endpointFixture, store: endpointStore)
            #expect(endpointFixture.count(playbackPath) == 1 && endpointFixture.count(preferencesPath) == 1)
            let player = try #require(coordinator.player)
            let nativeItem = try #require(player.currentItem)
            let actual = nativeItem.duration.seconds
            try #require(actual.isFinite && abs(actual - 12) < 0.002, "Endpoint verdict requires actual native HLS duration agreement")
            let restored = player.currentTime().seconds
            let mapped = try #require(coordinator.source?.compatible?.timeline.presentationTime(pending.seconds))
            let expected = 12 - remaining
            try #require(abs(mapped - expected) < 0.002, "Source mapping must match the independently known omitted opening")
            try #require(coordinator.usingCompatibility)
            #expect(restored >= expected - 0.1 && restored <= actual + 0.01, "Mapped pending endpoint must not restart at an interior position")
            let deadline = ContinuousClock.now.advanced(by: .seconds(3))
            while !(coordinator.completed && !coordinator.buffering && player.timeControlStatus != .waitingToPlayAtSpecifiedRate),
                  ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
            let nativeEnds = observedEnds.withLock { $0.filter { $0.0 == ObjectIdentifier(nativeItem) } }
            let nativeEndDuringStartup = nativeEnds.contains { $0.1 <= startupReturned }
            let firstNativeEndElapsed = nativeEnds.first.map { String(describing: start.duration(to: $0.1)) } ?? "none"
            print("HLS_EVENT_TIMING remaining=\(remaining) prepared=\(prepared) visible=\(visible) startupElapsed=\(start.duration(to: startupReturned)) firstNativeEndElapsed=\(firstNativeEndElapsed) nativeEndDuringStartup=\(nativeEndDuringStartup)")
            print("HLS_ENDPOINT remaining=\(remaining) prepared=\(prepared) visible=\(visible) beforePlaySource=\(beforePlaySource) beforePlayPreferences=\(beforePlayPreferences) afterPlaySource=\(endpointFixture.count(playbackPath)) afterPlayPreferences=\(endpointFixture.count(preferencesPath)) pendingSource=\(pending.seconds) mapped=\(mapped) actualDuration=\(actual) restored=\(restored) native=\(player.currentTime().seconds) completed=\(coordinator.completed) buffering=\(coordinator.buffering) rate=\(player.rate) timeControl=\(player.timeControlStatus.rawValue) completionCount=\(completions) nativeEndCount=\(nativeEnds.count) sameController=\(coordinator.presentation.controller === retainedController) attached=\(retainedController.view.window === window) elapsed=\(start.duration(to: .now))")
            #expect(coordinator.presentation.controller === retainedController)
            #expect((retainedController.view.window === window) == visible)
            if visible { #expect(retainedController.player === player) }
            else { #expect(retainedController.player == nil && retainedController.view.window == nil && !coordinator.presentation.visible) }
            #expect(coordinator.completed, "Mapped endpoint must complete instead of waiting indefinitely")
            #expect(!coordinator.buffering)
            #expect(player.timeControlStatus != .waitingToPlayAtSpecifiedRate)
            #expect(completions == 1)
            let progressDeadline = ContinuousClock.now.advanced(by: .seconds(3))
            var recorded = false
            var pendingAtVerdict: [PendingProgress] = []
            while ContinuousClock.now < progressDeadline {
                pendingAtVerdict = try await endpointStore.pending().filter { $0.itemID == endpointItem.id }
                recorded = watchedRecorded(pendingAtVerdict, fixture: endpointFixture, itemID: endpointItem.id, seconds: pending.seconds)
                if recorded { break }
                try await Task.sleep(for: .milliseconds(10))
            }
            let reopened = try await ProgressSyncStore(scope: endpointFixture.client.profileScope(), directory: endpointDirectory)
            let persisted = try await reopened.pending().filter { $0.itemID == endpointItem.id }
            let persistedRecorded = watchedRecorded(persisted, fixture: endpointFixture, itemID: endpointItem.id, seconds: pending.seconds)
            let receipts = endpointFixture.progressReceipts.filter { $0.itemID == endpointItem.id }
            print("HLS_PROGRESS remaining=\(remaining) prepared=\(prepared) visible=\(visible) committedAtEnd=\(committedAtEnd ?? -1) currentSource=\(coordinator.seconds) receiptCount=\(receipts.count) pendingCount=\(pendingAtVerdict.count) persistedRecorded=\(persistedRecorded)")
            for receipt in receipts { print("HLS_PROGRESS_RECEIPT seconds=\(receipt.seconds) watched=\(receipt.watched) revision=\(receipt.revision)") }
            for entry in persisted { print("HLS_PROGRESS_PENDING seconds=\(entry.progress.seconds) watched=\(entry.progress.watched) revision=\(entry.progress.revision) conflict=\(entry.conflict != nil)") }
            #expect(recorded && persistedRecorded && ContinuousClock.now <= progressDeadline, "Watched progress must persist locally or have positive synced receipt evidence within the original bound")
            await endpointFixture.client.close()
            await fixture.client.close()
        }

        private func fictionalStore(_ fixture: PlaybackStartupFixture, directory: URL? = nil) async throws -> ProgressSyncStore {
            let root = directory ?? FileManager.default.temporaryDirectory.appendingPathComponent("fictional-hls-endpoint-" + UUID().uuidString)
            return try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: root)
        }

        private func watchedRecorded(_ entries: [PendingProgress], fixture: PlaybackStartupFixture, itemID: String, seconds: Double) -> Bool {
            if let entry = entries.first {
                return entry.conflict == nil && entry.progress.watched && abs(entry.progress.seconds - seconds) < 0.1
            }
            return fixture.progressReceipts.contains { $0.itemID == itemID && $0.watched && abs($0.seconds - seconds) < 0.1 }
        }

        private func boundedPlay(_ coordinator: PlaybackCoordinator, item: MediaItem,
                                 fixture: PlaybackStartupFixture, store: ProgressSyncStore) async throws -> ContinuousClock.Instant {
            var result: Result<ContinuousClock.Instant, Error>?
            let task = Task { @MainActor in
                do { try await coordinator.play(item, client: fixture.client, store: store); result = .success(.now) }
                catch { result = .failure(error) }
            }
            defer { task.cancel() }
            let deadline = ContinuousClock.now.advanced(by: .seconds(12))
            while result == nil, ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
            try #require(result != nil, "Native HLS startup must finish within twelve seconds")
            let finished = try #require(result)
            return try finished.get()
        }
    }
}

@MainActor private struct HLSEndpointProbeSurface: View {
    let coordinator: PlaybackCoordinator
    var body: some View {
        if let player = coordinator.player {
            NativePlayerView(player: player, presentation: coordinator.presentation,
                             options: {}, seekPreview: {}, close: {}, restore: {}).ignoresSafeArea()
        } else { Color.black.ignoresSafeArea() }
    }
}
#endif
