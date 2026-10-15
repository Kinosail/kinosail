#if os(tvOS)
import AVKit
import Foundation
import SwiftUI
import Synchronization
import Testing
@testable import KinosailPlayer

@Suite(.serialized, .enabled(if: FileManager.default.fileExists(atPath: "/tmp/kinosail-player-buffer-bar.mp4")))
@MainActor struct TVPlayerSurfaceStabilityJourneys {
    @Test func systemPlayerStaysAttachedWhilePlaybackIsReplaced() async throws {
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
        let controller = session.player.presentation.controller
        let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: TVPlaybackSurface(failure: nil, retry: {}, close: {}).environment(session))
        window.makeKeyAndVisible()
        defer { window.isHidden = true; previous?.makeKey() }

        let item = try fixture.item("movie")
        try await session.player.play(item, client: fixture.client, store: store)
        let attachDeadline = ContinuousClock.now.advanced(by: .seconds(5))
        while controller.view.window !== window, ContinuousClock.now < attachDeadline { try await Task.sleep(for: .milliseconds(10)) }
        try #require(controller.view.window === window, "Prerequisite: the system player must be visible after playback starts")

        let detached = Mutex(0)
        let sampling = Task { @MainActor in
            while !Task.isCancelled {
                if controller.view.window !== window { detached.withLock { $0 += 1 } }
                try? await Task.sleep(for: .milliseconds(2))
            }
        }
        fixture.holdMedia = true
        let replacement = Task { try await session.player.play(item, client: fixture.client, store: store) }
        let holdDeadline = ContinuousClock.now.advanced(by: .seconds(5))
        while fixture.heldMediaCount == 0, ContinuousClock.now < holdDeadline { try await Task.sleep(for: .milliseconds(10)) }
        try await Task.sleep(for: .milliseconds(300))
        let pendingDetached = detached.withLock { $0 }
        fixture.releaseMedia()
        try await replacement.value
        try await Task.sleep(for: .milliseconds(300))
        let replacedDetached = detached.withLock { $0 }

        session.player.stop()
        try await Task.sleep(for: .milliseconds(300))
        sampling.cancel()
        print("TV_SURFACE pendingDetachedSamples=\(pendingDetached) replacedDetachedSamples=\(replacedDetached) heldMedia=\(fixture.heldMediaCount)")
        #expect(pendingDetached == 0, "The system player must stay attached while a replacement title is loading")
        #expect(replacedDetached == 0, "The system player must stay attached through the replacement")
        await fixture.client.close(purgeCache: true)
    }
}
#endif
