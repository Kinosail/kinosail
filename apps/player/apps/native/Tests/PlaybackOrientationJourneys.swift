#if os(iOS)
import AVKit
import SwiftUI
import Testing
@testable import KinosailPlayer

// Defined before production edits. Isolated UIKit/real decoded-media journeys;
// system Portrait Orientation Lock and physical iPhone behavior need device proof.
@Suite(.serialized) @MainActor
struct PlaybackOrientationJourneys {
    @Test func detachedLandscapeRequestIsRecoverable() {
        let orientation = PlaybackOrientation()
        orientation.toggle(in: nil)
        #expect(orientation.message != nil)
        #expect(!orientation.requesting)
        #expect(!orientation.landscapeRequested)
        orientation.restore()
        #expect(!orientation.requesting)
    }

    @Test func deniedRequestAndImmediateDismissalCannotLeaveStaleState() async throws {
        let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = PortraitOnlyController(rootView: Text("Orientation denial fixture"))
        window.makeKeyAndVisible()
        defer { window.isHidden = true; previous?.makeKey() }
        try await until { scene.activationState == .foregroundActive }
        scene.requestGeometryUpdate(.iOS(interfaceOrientations: .portrait))
        try await until { scene.effectiveGeometry.interfaceOrientation == .portrait }
        let orientation = PlaybackOrientation()
        orientation.toggle(in: window)
        orientation.toggle(in: window)
        try await until { !orientation.requesting }
        #expect(orientation.message != nil)
        #expect(!orientation.landscapeRequested)
        #expect(scene.effectiveGeometry.interfaceOrientation == .portrait)
        orientation.toggle(in: window)
        orientation.restore()
        try await until { !orientation.requesting }
        #expect(!orientation.landscapeRequested)
        #expect(orientation.message == nil, "A stale landscape error must not overwrite a newer successful return")
    }

    @Test(.enabled(if: FileManager.default.fileExists(atPath: "/tmp/kinosail-landscape-task10-media.mp4")))
    func landscapeReturnAndDismissalKeepDecodedPlaybackAndCaptureStates() async throws {
        let fixture = try await PlaybackStartupFixture()
        fixture.denied = false
        fixture.media = try Data(contentsOf: URL(fileURLWithPath: "/tmp/kinosail-landscape-task10-media.mp4"))
        defer { fixture.close() }
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        let store = try await ProgressSyncStore(scope: fixture.client.profileScope(), directory: folder)
        let session = AppSession()
        defer { session.player.stop(); try? FileManager.default.removeItem(at: folder) }
        try await session.player.play(fixture.item("movie"), client: fixture.client, store: store)
        let player = try #require(session.player.player)
        let item = try #require(player.currentItem)
        try await until { item.status == .readyToPlay && player.currentTime().seconds > 3.05 }
        session.player.pause()
        try await session.player.seek(to: 4)
        try await session.player.changeRate(1.25)
        let audio = session.player.selectedAudioTrackID
        let subtitle = session.player.selectedSubtitleTrackID
        let requests = fixture.count("/api/v1/items/movie/playback")
        let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: TouchPlaybackView(failure: nil, retry: {}, close: {})
            .environment(session))
        window.makeKeyAndVisible()
        defer { window.isHidden = true; previous?.makeKey() }
        try await until { scene.activationState == .foregroundActive && session.player.presentation.readyForDisplay }
        scene.requestGeometryUpdate(.iOS(interfaceOrientations: .portrait))
        try await until { scene.effectiveGeometry.interfaceOrientation == .portrait }
        session.player.presentation.showCaptions("Synthetic continuity caption")
        try await snapshot(window, name: "portrait-loaded")
        for attempt in 0..<2 {
            try await until { landscapeButton(window, label: "Watch in landscape")?.isEnabled == true }
            let button = try #require(descendants(window).compactMap { $0 as? UIButton }.first { $0.accessibilityLabel == "Watch in landscape" })
            #expect(button.bounds.width >= 44 && button.bounds.height >= 44)
            button.sendActions(for: .touchUpInside)
            button.sendActions(for: .touchUpInside)
            try await until {
                scene.effectiveGeometry.interfaceOrientation.isLandscape && descendants(window).compactMap { $0 as? UIButton }
                    .contains { $0.accessibilityLabel == "Return to previous orientation" && $0.isEnabled }
            }
            let returnButton = try #require(descendants(window).compactMap { $0 as? UIButton }.first { $0.accessibilityLabel == "Return to previous orientation" })
            #expect(session.player.player === player && player.currentItem === item)
            #expect(player.timeControlStatus == .paused)
            #expect(abs(player.currentTime().seconds - 4) < 0.15)
            #expect(session.player.playbackRate == 1.25)
            #expect(session.player.selectedAudioTrackID == audio)
            #expect(session.player.selectedSubtitleTrackID == subtitle)
            #expect(session.player.presentation.captionText == "Synthetic continuity caption")
            #expect(session.player.presentation.videoView.playerLayer.videoGravity == .resizeAspect)
            try await snapshot(window, name: "landscape-\(attempt)")
            if attempt == 0 { returnButton.sendActions(for: .touchUpInside) }
            else { window.rootViewController = UIHostingController(rootView: Text("Library after dismissal")) }
            try await until { scene.effectiveGeometry.interfaceOrientation == .portrait }
            #expect(scene.effectiveGeometry.interfaceOrientation == .portrait)
        }
        window.rootViewController = UIHostingController(rootView: TouchPlaybackView(failure: nil, retry: {}, close: {}).environment(session))
        try await until { session.player.presentation.readyForDisplay && landscapeButton(window, label: "Watch in landscape")?.isEnabled == true }
        let immediate = try #require(descendants(window).compactMap { $0 as? UIButton }.first { $0.accessibilityLabel == "Watch in landscape" })
        immediate.sendActions(for: .touchUpInside)
        window.rootViewController = UIHostingController(rootView: Text("Library during rotation"))
        try await Task.sleep(for: .milliseconds(500))
        try await until { scene.effectiveGeometry.interfaceOrientation == .portrait }
        window.rootViewController = UIHostingController(rootView: TouchPlaybackView(failure: nil, retry: {}, close: {}).environment(session))
        try await until { session.player.presentation.readyForDisplay && landscapeButton(window, label: "Watch in landscape")?.isEnabled == true }
        #expect(fixture.count("/api/v1/items/movie/playback") == requests)
        session.player.resume()
        try await until { player.currentTime().seconds > 4.15 }
        let moving = player.currentTime().seconds
        try #require(landscapeButton(window, label: "Watch in landscape")).sendActions(for: .touchUpInside)
        try await until { landscapeButton(window, label: "Return to previous orientation")?.isEnabled == true && scene.effectiveGeometry.interfaceOrientation.isLandscape }
        #expect(player.timeControlStatus == .playing && player.rate == 1.25)
        #expect(player.currentTime().seconds > moving)
        try await snapshot(window, name: "landscape-playing")
        try #require(landscapeButton(window, label: "Return to previous orientation")).sendActions(for: .touchUpInside)
        try await until { landscapeButton(window, label: "Watch in landscape")?.isEnabled == true && scene.effectiveGeometry.interfaceOrientation == .portrait }
        #expect(player.timeControlStatus == .playing && player.currentTime().seconds > moving)
        #expect(session.player.player === player)
        session.player.pause()
        try await snapshot(window, name: "portrait-returned")
        window.rootViewController = UIHostingController(rootView: TouchPlaybackView(failure: nil, retry: {}, close: {})
            .environment(session).environment(\.dynamicTypeSize, .accessibility3))
        try await snapshot(window, name: "loaded-large-text")
        window.rootViewController = UIHostingController(rootView: TouchPlaybackView(failure: "Synthetic playback failure", retry: {}, close: {})
            .environment(session).environment(\.dynamicTypeSize, .accessibility3))
        try await Task.sleep(for: .milliseconds(200))
        try await snapshot(window, name: "failed-large-text")
        let opening = AppSession()
        window.rootViewController = UIHostingController(rootView: TouchPlaybackView(failure: nil, retry: {}, close: {}).environment(opening))
        try await snapshot(window, name: "opening-empty")
        #expect(landscapeButton(window, label: "Watch in landscape")?.isEnabled == false)
        await fixture.client.close(purgeCache: true)
    }

    @Test(.enabled(if: FileManager.default.fileExists(atPath: "/tmp/kinosail-landscape-task10-media.mp4")))
    func closeReturnsThroughNavigationAndSavesPausedPosition() async throws {
        let fixture = try await PlaybackStartupFixture()
        fixture.denied = false
        fixture.media = try Data(contentsOf: URL(fileURLWithPath: "/tmp/kinosail-landscape-task10-media.mp4"))
        defer { fixture.close() }
        let keychain = SessionKeychain()
        let previousSession = try await keychain.restore()
        let session = AppSession()
        do {
            try await keychain.save(SavedSession(server: fixture.server, token: "fictional-startup-token", viewer: fixture.viewer))
            await session.restore()
            let client = try #require(session.client)
            let store = try #require(session.progress)
            defer { session.player.stop() }
            try await session.player.play(fixture.item("movie"), client: client, store: store)
            let player = try #require(session.player.player)
            try await until { player.currentItem?.status == .readyToPlay && player.currentTime().seconds > 3.05 }
            session.player.pause()
            try await session.player.seek(to: 5)
            let scene = try #require(UIApplication.shared.connectedScenes.compactMap { $0 as? UIWindowScene }.first)
            let previousWindow = scene.windows.first { $0.isKeyWindow }
            let window = UIWindow(windowScene: scene)
            let route = PlaybackJourneyRoute()
            window.rootViewController = UIHostingController(rootView: PlaybackJourneyNavigation(route: route).environment(session))
            window.makeKeyAndVisible()
            defer { window.isHidden = true; previousWindow?.makeKey() }
            try await until { session.player.presentation.readyForDisplay && landscapeButton(window, label: "Watch in landscape")?.isEnabled == true }
            try #require(landscapeButton(window, label: "Watch in landscape")).sendActions(for: .touchUpInside)
            try await until { scene.effectiveGeometry.interfaceOrientation.isLandscape && landscapeButton(window, label: "Return to previous orientation")?.isEnabled == true }
            let dismissal = PlaybackDismissal()
            // Exercise the accepted drag policy and the actual NavigationStack close
            // callback. Touch injection and physical hit testing remain device checks.
            dismissal.dragEnded(translation: CGSize(width: 5, height: 140), start: CGPoint(x: 80, y: 100),
                                size: window.bounds.size, excludedBottom: 144, close: { route.path = [] })
            dismissal.requestClose { route.path = [] }
            try await until { route.path.isEmpty && session.player.player == nil && scene.effectiveGeometry.interfaceOrientation == .portrait }
            try await until { fixture.savedSeconds.map { abs($0 - 5) < 0.15 } == true }
            #expect(player.timeControlStatus == .paused)
            try await snapshot(window, name: "dismissed-library")
            await client.close(purgeCache: true)
            await fixture.client.close(purgeCache: true)
        } catch {
            try await restore(previousSession, keychain: keychain, session: session)
            throw error
        }
        try await restore(previousSession, keychain: keychain, session: session)
    }

    private func restore(_ previous: SavedSession?, keychain: SessionKeychain, session: AppSession) async throws {
        session.player.stop()
        await session.client?.close(purgeCache: true)
        if let previous { try await keychain.save(previous) } else { try await keychain.clear() }
    }

    private func until(_ condition: () -> Bool) async throws {
        for _ in 0..<300 {
            if condition() { return }
            try await Task.sleep(for: .milliseconds(20))
        }
        Issue.record("Orientation journey did not reach expected geometry/playback state")
        throw CancellationError()
    }

    private func snapshot(_ window: UIWindow, name: String) async throws {
        try await until {
            guard let scene = window.windowScene else { return false }
            return scene.effectiveGeometry.interfaceOrientation.isLandscape == (window.bounds.width > window.bounds.height)
        }
        // Scene geometry changes before SwiftUI's rotation animation finishes.
        try await Task.sleep(for: .milliseconds(450))
        window.layoutIfNeeded()
        let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in window.drawHierarchy(in: window.bounds, afterScreenUpdates: true) }
        let folder = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].appendingPathComponent("orientation-artifacts")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        try #require(image.pngData()).write(to: folder.appendingPathComponent(name + ".png"))
        print("ORIENTATION capture=\(name) size=\(window.bounds.size) physicalLockProof=false")
    }

    private func descendants(_ view: UIView) -> [UIView] { view.subviews.flatMap { [$0] + descendants($0) } }

    private func landscapeButton(_ window: UIWindow, label: String) -> UIButton? {
        descendants(window).compactMap { $0 as? UIButton }.first { $0.accessibilityLabel == label }
    }
}

private final class PortraitOnlyController<Content: View>: UIHostingController<Content> {
    override var supportedInterfaceOrientations: UIInterfaceOrientationMask { .portrait }
}

@MainActor @Observable private final class PlaybackJourneyRoute { var path = ["movie"] }

private struct PlaybackJourneyNavigation: View {
    @Bindable var route: PlaybackJourneyRoute
    var body: some View {
        NavigationStack(path: $route.path) {
            Text("Library after closing video")
                .navigationDestination(for: String.self) { id in
                    PlaybackScreen(itemID: id, onClose: { route.path = [] })
                }
        }
    }
}
#endif
