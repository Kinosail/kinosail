#if os(tvOS)
import AVFoundation
import SwiftUI
import Testing
@testable import KinosailPlayer

// Test-first gap: real journals and native EOF/actor ordering cannot be seeded
// safely. MP4/HLS decode uses real AVFoundation; explicit EOF posts below are
// isolated notification ownership tests, not natural EOF or Nox proof.
extension PlaybackEndpointJourneys {
    @Suite
    @MainActor struct PlaybackCompletionSafetyJourneys {
        // The populated Nox journal is intentionally not seeded or read. This
        // fixture covers the public replay decision with real native decoding.
        @Test(arguments: ["device-watched", "server-endpoint", "server-resumed", "device-resumed"])
        func reopeningCompletedTitleUsesNewestPlaybackIntent(origin: String) async throws {
            let fixture = try await mp4Fixture()
            defer { fixture.close() }
            var item = try fixture.item("movie")
            if origin != "device-watched" {
                item.progress.seconds = 12; item.progress.watched = true
                item.progress.session = UUID().uuidString; item.progress.revision = 1
            }
            if origin == "server-endpoint" {
                fixture.playbackResponse = Data("{\"media\":{\"duration\":12},\"plan\":{\"allowed\":true,\"mode\":\"direct\",\"reason\":\"direct-preferred\"},\"duration\":12,\"start\":12,\"directAllowed\":true,\"direct\":\"/media/movie\",\"directType\":\"video/mp4\"}".utf8)
            }
            let folder = FileManager.default.temporaryDirectory.appendingPathComponent("fictional-watched-replay-" + UUID().uuidString)
            let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
            defer { try? FileManager.default.removeItem(at: folder) }
            if origin == "device-watched" || origin == "device-resumed" {
                var progress = WatchProgress()
                progress.seconds = origin == "device-watched" ? 12 : 3
                progress.watched = origin == "device-watched"
                progress.session = UUID().uuidString; progress.revision = 1
                try await store.record(itemID: item.id, progress: progress, expected: item.progress)
            }
            let coordinator = PlaybackCoordinator()
            defer { coordinator.onCompleted = nil; coordinator.stop() }
            var completions = 0
            coordinator.onCompleted = { _, _ in completions += 1 }
            try await joinedStartup(coordinator, item: item, fixture: fixture, store: store)
            let player = try #require(coordinator.player)
            let expected = origin.hasSuffix("resumed") ? 3.0 : 0.0
            let initial = player.currentTime().seconds
            #expect(initial.isFinite && initial >= expected && initial < expected + 1,
                    "Completed progress must replay; newer server or device resume intent must survive stale item metadata")
            #expect(completions == 0 && !coordinator.completed)
            try await decodedProgress(player)
            #expect(completions == 0 && !coordinator.completed)
            print("COMPLETION_SAFETY case=watched-replay origin=\(origin) initial=\(initial) expected=\(expected) decodedAdvancing=true callbackCount=\(completions)")
            await fixture.client.close()
        }

        @Test(arguments: [0.0, 0.024])
        func naturalMP4CompletionCallbackCanStopWithoutRestarting(remaining: Double) async throws {
            let fixture = try await mp4Fixture()
            defer { fixture.close() }
            let item = try fixture.item("movie")
            let store = try await pendingStore(fixture, seconds: 12 - remaining)
            let coordinator = PlaybackCoordinator()
            defer { coordinator.onCompleted = nil; coordinator.stop() }
            var completions = 0
            var stoppedPlayer: AVPlayer?
            var committed: Double?
            coordinator.onCompleted = { ended, _ in
                guard ended.id == item.id else { return }
                completions += 1; stoppedPlayer = coordinator.player; committed = coordinator.seconds
                coordinator.stop()
            }
            do { try await joinedStartup(coordinator, item: item, fixture: fixture, store: store) }
            catch is CancellationError { try #require(completions == 1 && stoppedPlayer != nil, "Only callback-owned stop may cancel startup") }
            let deadline = ContinuousClock.now.advanced(by: .seconds(3))
            while completions == 0, ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
            try #require(completions == 1)
            let position = try #require(committed)
            let nativeItem = try #require(stoppedPlayer?.currentItem)
            try #require(nativeItem.duration.seconds.isFinite && abs(nativeItem.duration.seconds - 12) < 0.002, "Prerequisite: native MP4 duration matches declared fixture")
            try #require(abs(position - (12 - remaining)) < 0.1)
            #expect(coordinator.player == nil && coordinator.currentItem == nil)
            #expect(!coordinator.loading && !coordinator.buffering && !coordinator.isPlaying)
            #expect(stoppedPlayer?.rate == 0)
            try await watchedReceipt(fixture, itemID: item.id, seconds: position)
            print("COMPLETION_SAFETY case=callback-stop remaining=\(remaining) callbackCount=\(completions) committed=\(position) stopped=\(coordinator.player == nil && stoppedPlayer?.rate == 0) watchedReceipt=\(fixture.progressReceipts.contains { $0.itemID == item.id && $0.watched && abs($0.seconds - position) < 0.1 }) startupJoined=true")
            await fixture.client.close()
        }

        @Test func duplicateEOFPublishesOnceAndKeepsNativePaused() async throws {
            let fixture = try await mp4Fixture()
            defer { fixture.close() }
            let item = try fixture.item("movie")
            let store = try await pendingStore(fixture, seconds: 3)
            let coordinator = PlaybackCoordinator()
            defer { coordinator.onCompleted = nil; coordinator.stop() }
            var completions = 0
            var committed: Double?
            coordinator.onCompleted = { ended, _ in
                if ended.id == item.id {
                    if completions == 0 { committed = coordinator.seconds }
                    completions += 1
                }
            }
            try await joinedStartup(coordinator, item: item, fixture: fixture, store: store)
            let player = try #require(coordinator.player)
            try await decodedProgress(player)
            let nativeItem = try #require(player.currentItem)
            NotificationCenter.default.post(name: .AVPlayerItemDidPlayToEndTime, object: nativeItem)
            NotificationCenter.default.post(name: .AVPlayerItemDidPlayToEndTime, object: nativeItem)
            let deadline = ContinuousClock.now.advanced(by: .seconds(3))
            while completions == 0, ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
            try await Task.sleep(for: .milliseconds(100))
            #expect(completions == 1)
            #expect(coordinator.completed && !coordinator.isPlaying && !coordinator.buffering)
            #expect(player.rate == 0 && player.timeControlStatus == .paused)
            let position = try #require(committed)
            try #require(position.isFinite && position > 0 && position < 12)
            try await watchedReceipt(fixture, itemID: item.id, seconds: position)
            print("COMPLETION_SAFETY case=injected-duplicate callbackCount=\(completions) paused=\(player.rate == 0 && player.timeControlStatus == .paused) watchedReceipt=\(fixture.progressReceipts.contains { $0.itemID == item.id && $0.watched && abs($0.seconds - position) < 0.1 })")
            await fixture.client.close()
        }

        @Test func queuedAndLatePreviousItemEOFCannotCompleteReopenedInterior() async throws {
            let fixture = try await mp4Fixture()
            defer { fixture.close() }
            let item = try fixture.item("movie")
            let coordinator = PlaybackCoordinator()
            defer { coordinator.onCompleted = nil; coordinator.stop() }
            var completions = 0
            coordinator.onCompleted = { _, _ in completions += 1 }
            let firstStore = try await pendingStore(fixture, seconds: 3)
            try await joinedStartup(coordinator, item: item, fixture: fixture, store: firstStore)
            let first = try #require(coordinator.player)
            try await decodedProgress(first)
            let oldItem = try #require(first.currentItem)
            NotificationCenter.default.post(name: .AVPlayerItemDidPlayToEndTime, object: oldItem)
            coordinator.stop() // Invalidate queued actor work before yielding.
            let secondStore = try await pendingStore(fixture, seconds: 3)
            try await joinedStartup(coordinator, item: item, fixture: fixture, store: secondStore)
            let second = try #require(coordinator.player)
            try #require(second !== first && second.currentItem !== oldItem)
            NotificationCenter.default.post(name: .AVPlayerItemDidPlayToEndTime, object: oldItem)
            try await decodedProgress(second)
            #expect(completions == 0 && !coordinator.completed)
            #expect(!fixture.progressReceipts.contains { $0.itemID == item.id && $0.watched })
            print("COMPLETION_SAFETY case=injected-stale callbackCount=\(completions) interiorAdvancing=true watchedReceipt=\(fixture.progressReceipts.contains { $0.itemID == item.id && $0.watched })")
            await fixture.client.close()
        }

        @Test func nativeScrubSupersedesEndpointRestoreWithoutWatchedCompletion() async throws {
            let fixture = try await PlaybackEndpointJourneys.PlaybackHLSEndpointJourneys.compatibleFixture()
            defer { fixture.close() }
            let item = try fixture.item("movie")
            let coordinator = PlaybackCoordinator()
            defer { coordinator.onCompleted = nil; coordinator.stop() }
            let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
            let previous = scene.windows.first { $0.isKeyWindow }
            let window = UIWindow(windowScene: scene)
            window.rootViewController = UIHostingController(rootView: CompletionSafetySurface(coordinator: coordinator))
            window.makeKeyAndVisible()
            defer { window.isHidden = true; previous?.makeKey() }
            var superseded = false
            var completions = 0
            coordinator.onCompleted = { _, _ in completions += 1 }
            let observation = CompletionScrubObservation()
            defer { observation.record() }
            let observer = NotificationCenter.default.addObserver(forName: .AVPlayerItemDidPlayToEndTime, object: nil, queue: .main) { notification in
                guard let ended = notification.object as? AVPlayerItem else { return }
                MainActor.assumeIsolated {
                    guard !superseded, coordinator.loading, let player = coordinator.player,
                          ended === player.currentItem,
                          coordinator.presentation.controller.player === player,
                          coordinator.presentation.controller.view.window === window else { return }
                    superseded = true
                    _ = coordinator.presentation.playerViewController(coordinator.presentation.controller,
                        timeToSeekAfterUserNavigatedFrom: player.currentTime(), to: CMTime(seconds: 3, preferredTimescale: 600))
                    observation.observe("scrub-injected", coordinator: coordinator, completions: completions)
                }
            }
            defer { NotificationCenter.default.removeObserver(observer) }
            let store = try await pendingStore(fixture, seconds: 15)
            try await joinedStartup(coordinator, item: item, fixture: fixture, store: store,
                                    onReturn: { observation.observe("startup-return", coordinator: coordinator, completions: completions) })
            try #require(superseded, "Prerequisite: native endpoint EOF must invoke a scrub while owned AVKit is attached and loading")
            let player = try #require(coordinator.player)
            let nativeSeconds = player.currentTime().seconds, sourceSeconds = coordinator.seconds
            observation.observe("assertion", coordinator: coordinator, completions: completions,
                                native: nativeSeconds, source: sourceSeconds)
            try #require(abs(nativeSeconds - 3) < 0.1 && abs(sourceSeconds - 6) < 0.1)
            try await decodedProgress(player)
            #expect(completions == 0 && !coordinator.completed)
            #expect(!fixture.progressReceipts.contains { $0.itemID == item.id && $0.watched })
            print("COMPLETION_SAFETY case=actual-EOF-superseded nativeTarget=3 sourceTarget=6 interiorAdvancing=true callbackCount=\(completions) watchedReceipt=\(fixture.progressReceipts.contains { $0.itemID == item.id && $0.watched })")
            await fixture.client.close()
        }

        private func mp4Fixture() async throws -> PlaybackStartupFixture {
            let data = try PlaybackEndpointMedia.data(.source)
            try #require(data.count <= 1024 * 1024)
            let fixture = try await PlaybackStartupFixture()
            fixture.denied = false; fixture.media = data
            return fixture
        }

        private func pendingStore(_ fixture: PlaybackStartupFixture, seconds: Double) async throws -> ProgressSyncStore {
            let folder = FileManager.default.temporaryDirectory.appendingPathComponent("fictional-completion-safety-" + UUID().uuidString)
            let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
            var progress = WatchProgress()
            progress.seconds = seconds; progress.session = UUID().uuidString; progress.revision = 1
            try await store.record(itemID: "movie", progress: progress, expected: WatchProgress())
            return store
        }

        private func joinedStartup(_ coordinator: PlaybackCoordinator, item: MediaItem,
                                   fixture: PlaybackStartupFixture, store: ProgressSyncStore,
                                   onReturn: (@MainActor () -> Void)? = nil) async throws {
            var result: Result<Void, Error>?
            let task = Task { @MainActor in
                do {
                    try await coordinator.play(item, client: fixture.client, store: store)
                    onReturn?()
                    result = .success(())
                }
                catch { result = .failure(error) }
            }
            do {
                let deadline = ContinuousClock.now.advanced(by: .seconds(12))
                while result == nil, ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
                let finished = try #require(result, "Native startup must return within twelve seconds")
                await task.value
                try finished.get()
            } catch {
                task.cancel()
                await task.value
                throw error
            }
        }

        private func decodedProgress(_ player: AVPlayer) async throws {
            let item = try #require(player.currentItem)
            try #require(item.duration.seconds.isFinite && abs(item.duration.seconds - 12) < 0.002, "Prerequisite: native duration matches declared fixture")
            let output = AVPlayerItemVideoOutput(pixelBufferAttributes: [kCVPixelBufferPixelFormatTypeKey as String: kCVPixelFormatType_32BGRA])
            item.add(output)
            let initial = player.currentTime().seconds
            let deadline = ContinuousClock.now.advanced(by: .seconds(3))
            var decoded = false
            while ContinuousClock.now < deadline {
                if player.currentTime().seconds > initial + 0.1,
                   output.copyPixelBuffer(forItemTime: player.currentTime(), itemTimeForDisplay: nil) != nil { decoded = true; break }
                try await Task.sleep(for: .milliseconds(10))
            }
            try #require(decoded, "Prerequisite: actual native interior must decode and advance")
        }

        private func watchedReceipt(_ fixture: PlaybackStartupFixture, itemID: String, seconds: Double) async throws {
            let deadline = ContinuousClock.now.advanced(by: .seconds(3))
            while !fixture.progressReceipts.contains(where: { $0.itemID == itemID && $0.watched && abs($0.seconds - seconds) < 0.1 }),
                  ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
            try #require(fixture.progressReceipts.contains { $0.itemID == itemID && $0.watched && abs($0.seconds - seconds) < 0.1 })
        }
    }
}

@MainActor private final class CompletionScrubObservation {
    private let start = ProcessInfo.processInfo.systemUptime
    private var rows: [[String: Any]] = []

    func observe(_ phase: String, coordinator: PlaybackCoordinator, completions: Int,
                 native: Double? = nil, source: Double? = nil) {
        guard rows.count < 3 else { return }
        rows.append(["phase": phase, "elapsedMs": (ProcessInfo.processInfo.systemUptime - start) * 1000,
            "nativeSeconds": boundedPosition(native ?? coordinator.player?.currentTime().seconds),
            "sourceSeconds": boundedPosition(source ?? coordinator.seconds),
            "rate": boundedPosition(coordinator.player.map { Double($0.rate) }),
            "timeControlStatus": coordinator.player?.timeControlStatus.rawValue ?? -1,
            "loading": coordinator.loading, "completed": coordinator.completed, "completionCount": completions])
    }

    func record() {
        if let data = try? JSONSerialization.data(withJSONObject: rows, options: [.sortedKeys]) {
            Attachment.record(data, named: "native-scrub-startup.json")
            print("NATIVE_SCRUB_STARTUP " + String(decoding: data, as: UTF8.self))
        }
    }

    private func boundedPosition(_ value: Double?) -> Any {
        guard let value, value.isFinite, abs(value) <= 3600 else { return NSNull() }
        return value
    }
}

@MainActor private struct CompletionSafetySurface: View {
    let coordinator: PlaybackCoordinator
    var body: some View {
        if let player = coordinator.player {
            NativePlayerView(player: player, presentation: coordinator.presentation,
                             options: {}, seekPreview: {}, close: {}, restore: {}).ignoresSafeArea()
        } else { Color.black.ignoresSafeArea() }
    }
}
#endif
