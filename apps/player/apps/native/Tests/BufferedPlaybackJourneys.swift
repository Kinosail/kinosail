import AVFoundation
import Foundation
import SwiftUI
import Testing
import UIKit
@testable import KinosailPlayer

// Real decoded media crosses the loopback Server, authenticated transport,
// AVPlayer, observable playback state, and the app's actual playback screen.
@Suite(.serialized, .enabled(if: FileManager.default.fileExists(atPath: "/tmp/kinosail-player-buffer-bar.mp4")))
@MainActor struct BufferedPlaybackJourneys {
    @Test func decodedMediaBuffersReachThePlaybackScreenAndClearOnStop() async throws {
        #if os(iOS)
        try await journey(audio: false)
        #else
        try await journey(audio: true)
        #endif
    }

    #if os(iOS)
    @Test func decodedMediaBuffersReachTheAudioScreenAndClearOnStop() async throws {
        try await journey(audio: true)
    }
    #endif

    private func journey(audio: Bool) async throws {
        let media = try Data(contentsOf: URL(fileURLWithPath: "/tmp/kinosail-player-buffer-bar.mp4"))
        try #require(media.count <= 1024 * 1024)
        let fixture = try await PlaybackStartupFixture()
        fixture.denied = false; fixture.media = media
        defer { fixture.close() }
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: folder) }
        let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
        let session = AppSession()
        defer { session.player.stop() }
        #if os(iOS)
        if !audio { try await render(TouchPlaybackView(failure: nil, retry: {}, close: {}).environment(session), name: "pending", widths: [390, 1024]) }
        #endif
        let item = try (audio ? MediaItem(.object(["id": .string("music"), "kind": .string("music"), "title": .string("Buffer fixture")]), server: fixture.server) : fixture.item("movie"))
        try await session.player.play(item, client: fixture.client, store: store)
        for _ in 0..<500 where session.player.bufferedRanges.isEmpty { try await Task.sleep(for: .milliseconds(10)) }
        #expect(!session.player.bufferedRanges.isEmpty)
        #expect(session.player.player?.currentItem?.status == .readyToPlay)
        #expect(session.player.bufferedRanges.allSatisfy { $0.lowerBound >= 0 && $0.upperBound <= session.player.duration })
        session.player.pause()
        let position = session.player.seconds
        try await Task.sleep(for: .milliseconds(1100))
        #expect(!session.player.bufferedRanges.isEmpty)
        #expect(abs(session.player.seconds - position) < 0.1)
        #if os(iOS)
        if audio {
            try await render(AudioPlayerScreen(itemID: "music").environment(session), name: "loaded-audio", widths: [390, 1024])
        } else {
            try await render(TouchPlaybackView(failure: nil, retry: {}, close: {}).environment(session), name: "loaded-video", widths: [320, 390, 844, 1024])
            try await render(TouchPlaybackView(failure: "This media could not be played.", retry: {}, close: {}).environment(session), name: "failed", widths: [390, 1024])
        }
        #else
        try await render(AudioPlayerScreen(itemID: "music").environment(session), name: "loaded-audio", widths: [1920])
        try await render(TVSeekPreviewScreen().environment(session), name: "loaded-seek", widths: [1920])
        // The same decoded fixture contains video, so retain its paused player
        // while presenting AVKit's system controls.
        let player = try #require(session.player.player)
        let tracks = try await #require(player.currentItem).asset.loadTracks(withMediaType: .video)
        #expect(!tracks.isEmpty)
        #expect(!session.player.bufferedRanges.isEmpty)
        try await render(NativePlayerView(player: player, presentation: session.player.presentation,
            options: {}, seekPreview: {}, close: {}, restore: {}), name: "loaded-system-video", widths: [1920])
        #endif
        print("BUFFER real-media ranges=\(session.player.bufferedRanges) paused-position=\(position)")
        session.player.stop()
        #expect(session.player.bufferedRanges.isEmpty)
        await fixture.client.close(purgeCache: true)
    }

    private func render(_ view: some View, name: String, widths: [Double]) async throws {
        let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: view)
        window.makeKeyAndVisible()
        defer { window.isHidden = true; previous?.makeKey() }
        let folder = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("buffer-bar-proof")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        for width in widths {
            window.frame = CGRect(x: 0, y: 0, width: width, height: width >= 1024 ? 1080 : width == 844 ? 390 : 844)
            window.setNeedsLayout(); window.layoutIfNeeded()
            try await Task.sleep(for: .milliseconds(200))
            let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in window.drawHierarchy(in: window.bounds, afterScreenUpdates: true) }
            try #require(image.pngData()).write(to: folder.appendingPathComponent("\(name)-\(Int(width)).png"))
        }
    }
}
