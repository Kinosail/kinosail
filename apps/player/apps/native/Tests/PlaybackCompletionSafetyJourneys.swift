#if os(tvOS)
import AVFoundation
import SwiftUI
import Testing
@testable import KinosailPlayer

private let completionMediaFolder = "/Users/mikeo/Documents/Codex/2026-10-10/task/evidence/nox-random-50-20261010/near-end-resume/hls-fixture-2"

// Test-first gap: real journals and native EOF/actor ordering cannot be seeded
// safely. MP4/HLS decode uses real AVFoundation; explicit EOF posts below are
// isolated notification ownership tests, not natural EOF or Nox proof.
@Suite(.serialized, .enabled(if: FileManager.default.fileExists(atPath: completionMediaFolder + "/source.mp4")))
@MainActor struct PlaybackCompletionSafetyJourneys {
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
        let fixture = try await PlaybackHLSEndpointJourneys.compatibleFixture()
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
            }
        }
        defer { NotificationCenter.default.removeObserver(observer) }
        let store = try await pendingStore(fixture, seconds: 15)
        try await joinedStartup(coordinator, item: item, fixture: fixture, store: store)
        try #require(superseded, "Prerequisite: native endpoint EOF must invoke a scrub while owned AVKit is attached and loading")
        let player = try #require(coordinator.player)
        try #require(abs(player.currentTime().seconds - 3) < 0.1 && abs(coordinator.seconds - 6) < 0.1)
        try await decodedProgress(player)
        #expect(completions == 0 && !coordinator.completed)
        #expect(!fixture.progressReceipts.contains { $0.itemID == item.id && $0.watched })
        print("COMPLETION_SAFETY case=actual-EOF-superseded nativeTarget=3 sourceTarget=6 interiorAdvancing=true callbackCount=\(completions) watchedReceipt=\(fixture.progressReceipts.contains { $0.itemID == item.id && $0.watched })")
        await fixture.client.close()
    }

    private func mp4Fixture() async throws -> PlaybackStartupFixture {
        let data = try Data(contentsOf: URL(fileURLWithPath: completionMediaFolder + "/source.mp4"))
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
                               fixture: PlaybackStartupFixture, store: ProgressSyncStore) async throws {
        var result: Result<Void, Error>?
        let task = Task { @MainActor in
            do { try await coordinator.play(item, client: fixture.client, store: store); result = .success(()) }
            catch { result = .failure(error) }
        }
        defer { task.cancel() }
        let deadline = ContinuousClock.now.advanced(by: .seconds(12))
        while result == nil, ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(10)) }
        let finished = try #require(result, "Native startup must return within twelve seconds")
        await task.value
        try finished.get()
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
