import AVFoundation
import Foundation
import Testing
@testable import KinosailPlayer

// Failure cases: resume must not claim playback before media is available;
// changing speed must reach a waiting player, preserve a pause, and reject
// invalid rates before changing either player or persisted device choices.
@MainActor struct PlaybackBufferingTests {
    @Test func resumeKeepsPendingMediaInBufferingState() throws {
        let engine = try pendingEngine()
        defer { stop(engine) }
        engine.preferences.rate = 2
        engine.player?.defaultRate = 2

        engine.resume()

        #expect(engine.player?.rate == 2)
        #expect(engine.player?.timeControlStatus == .waitingToPlayAtSpecifiedRate)
        #expect(!engine.isPlaying)
        #expect(engine.buffering)
        #expect(engine.wantsPlayback)
        engine.togglePlayback()
        #expect(engine.player?.rate == 0)
        #expect(!engine.wantsPlayback)
        #expect(!engine.buffering)

        engine.player = nil
        engine.recoveringNetwork = true
        engine.resume()
        #expect(!engine.isPlaying)
        #expect(engine.buffering)
        engine.togglePlayback()
        #expect(!engine.wantsPlayback)
        #expect(engine.nativeIntent.playing.withLock { $0 } == false)
    }

    @Test(arguments: [0.5, 1, 2, 3], [false, true])
    func changingSpeedUpdatesWaitingPlaybackAndPreservesPause(_ rate: Double, _ paused: Bool) async throws {
        let engine = try pendingEngine()
        defer { stop(engine) }
        if !paused {
            engine.player?.play()
            engine.buffering = true
            #expect(engine.player?.timeControlStatus == .waitingToPlayAtSpecifiedRate)
        }

        try await engine.changeRate(rate)

        #expect(engine.playbackRate == rate)
        #expect(engine.player?.defaultRate == Float(rate))
        #expect(engine.player?.rate == (paused ? 0 : Float(rate)))
        #expect(engine.player?.timeControlStatus == (paused ? .paused : .waitingToPlayAtSpecifiedRate))
    }

    @Test func applyingPreferencesUpdatesWaitingPlaybackSpeed() async throws {
        let engine = try pendingEngine()
        defer { stop(engine) }
        engine.player?.play()
        engine.buffering = true
        var preferences = PlaybackPreferences()
        preferences.rate = 2

        try await engine.applyPreferences(preferences)

        #expect(engine.player?.rate == 2)
        #expect(engine.player?.defaultRate == 2)
        #expect(engine.player?.timeControlStatus == .waitingToPlayAtSpecifiedRate)
    }

    @Test(arguments: [Double.nan, .infinity, -.infinity, -1, 0, 0.49, 3.01])
    func rejectedSpeedPreservesWaitingPlayerAndSavedChoice(_ rate: Double) async throws {
        let engine = try pendingEngine()
        let scope = try #require(engine.devicePreferencesScope)
        defer { stop(engine) }
        engine.player?.play()
        engine.buffering = true

        await #expect(throws: ClientError.self) { try await engine.changeRate(rate) }

        #expect(engine.playbackRate == 1)
        #expect(engine.preferences.rate == 1)
        #expect(engine.player?.rate == 1)
        #expect(engine.player?.defaultRate == 1)
        #expect(UserDefaults.standard.object(forKey: DevicePlaybackChoices.key(scope: scope)) == nil)
    }

    private static let loader = PendingMediaLoader()

    private func pendingEngine() throws -> PlaybackEngine {
        let engine = PlaybackEngine()
        let asset = AVURLAsset(url: URL(string: "kinosail-buffer-fixture://media/pending.mp4")!)
        asset.resourceLoader.setDelegate(Self.loader, queue: DispatchQueue(label: "buffer-fixture"))
        engine.player = AVPlayer(playerItem: AVPlayerItem(asset: asset))
        engine.currentItem = try MediaItem(.object(["id": .string("movie"), "kind": .string("video"), "title": .string("Fixture")]),
                                          server: ServerAddress("https://media.example.invalid"))
        engine.devicePreferencesScope = UUID().uuidString
        return engine
    }

    private func stop(_ engine: PlaybackEngine) {
        if let scope = engine.devicePreferencesScope { UserDefaults.standard.removeObject(forKey: DevicePlaybackChoices.key(scope: scope)) }
        engine.stop()
    }
}

private final class PendingMediaLoader: NSObject, AVAssetResourceLoaderDelegate {
    func resourceLoader(_ resourceLoader: AVAssetResourceLoader, shouldWaitForLoadingOfRequestedResource loadingRequest: AVAssetResourceLoadingRequest) -> Bool { true }
}
