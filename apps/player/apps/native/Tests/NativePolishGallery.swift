#if os(iOS)
import SwiftUI
import UIKit
import WebKit
@testable import KinosailPlayer

/// Render evidence accompanies the native control regression, using fictional
/// library data served over loopback and the production SwiftUI screens.
@MainActor enum NativePolishGallery {
    static func record(session: AppSession, fixture: PreferencesLoopbackFixture) async throws {
        let mode = ProcessInfo.processInfo.environment["KINOSAIL_POLISH_GALLERY"]
        guard ["1", "confirm", "lifecycle"].contains(mode),
              let scene = UIApplication.shared.connectedScenes.first as? UIWindowScene else { return }
        fixture.artwork = cover()
        fixture.landscape = cover(wide: true)
        fixture.populated = true
        await session.client?.invalidateCatalog()
        session.resourceSnapshots.clear()
        if mode == "lifecycle" {
            let views: [(String, AnyView)] = [("home", AnyView(HomeScreen(selectTab: { _ in }))),
                ("library", AnyView(LibraryScreen(initialView: .shows)))]
            for (state, delay, populated, fails) in [("pending", 1.5, true, false), ("loaded", 0.0, true, false),
                                                     ("empty", 0.0, false, false), ("failed", 0.0, true, true)] {
                fixture.delay = delay; fixture.populated = populated; fixture.fails = fails
                if state == "failed" { await session.client?.discardMediaCache() }
                for (name, view) in views {
                    await session.client?.invalidateCatalog()
                    session.resourceSnapshots.clear()
                    try await render(AnyView(view.environment(\.scenePhase, .inactive)), name: name + "-" + state,
                                     scene: scene, session: session, wait: state == "pending" ? 0.2 : state == "failed" ? 1.5 : 0.7)
                }
            }
            fixture.delay = 0; fixture.populated = true; fixture.fails = false
            return
        }
        let screens: [(String, AnyView)] = [
            ("home", AnyView(HomeScreen(selectTab: { _ in }))),
            ("movies", AnyView(LibraryScreen(initialView: .movies))),
            ("search", AnyView(LibraryScreen(searchMode: true))),
            ("movie", AnyView(DetailScreen(itemID: "movie"))),
            ("episodes", AnyView(ShowScreen(showID: PreferencesLoopbackFixture.showID))),
            ("albums", AnyView(MusicScreen())), ("album", AnyView(AlbumScreen(albumID: "album"))),
            ("collections", AnyView(CollectionsScreen())), ("collection", AnyView(CollectionScreen(name: "Sunday discoveries"))),
            ("settings", AnyView(SettingsScreen())), ("playback-preferences", AnyView(PlaybackPreferencesScreen())),
            ("download-preferences", AnyView(OfflinePreferencesScreen())), ("reader-preferences", AnyView(ReaderPreferencesScreen())),
            ("tabs", AnyView(TabPreferencesScreen())), ("supporter", AnyView(SupporterScreen())),
            ("approval", AnyView(ApprovalScreen(showsDismiss: true))), ("cast", AnyView(PlayOnTVScreen(itemID: "movie"))),
            ("downloads-empty", AnyView(DownloadsScreen())), ("bookmarks-empty", AnyView(BookmarksScreen(itemID: "movie"))),
            ("progress-sync", AnyView(ProgressSyncScreen())),
            ("reader", AnyView(ReaderScreen(itemID: "book"))), ("photo", AnyView(PhotoScreen(itemID: "photo")))
        ]
        for (name, screen) in screens where mode != "confirm" || ["reader", "approval", "cast"].contains(name) {
            try await render(screen, name: name, scene: scene, session: session)
        }
        if mode == "confirm" { return }
        for (name, screen) in screens.filter({ ["settings", "playback-preferences", "download-preferences", "reader-preferences", "tabs", "supporter", "episodes"].contains($0.0) }) {
            try await render(AnyView(screen.environment(\.dynamicTypeSize, .accessibility3)), name: name + "-large-text", scene: scene, session: session)
        }
        // Pending and terminal states use an uncached operation, so the Server
        // really is outstanding when the placeholder is captured.
        fixture.delay = 1.5
        try await render(AnyView(SupporterScreen()), name: "supporter-pending", scene: scene, session: session, wait: 0.2)
        fixture.delay = 0
        fixture.fails = true
        try await render(AnyView(SupporterScreen()), name: "supporter-failed", scene: scene, session: session)
        fixture.fails = false
        fixture.populated = false
        try await render(AnyView(SupporterScreen()), name: "supporter-empty", scene: scene, session: session)
        let disconnected = AppSession()
        for (name, screen) in [("preferences-failed", AnyView(OfflinePreferencesScreen())), ("reader-failed", AnyView(ReaderScreen(itemID: "book"))),
                               ("photo-failed", AnyView(PhotoScreen(itemID: "photo"))), ("bookmarks-failed", AnyView(BookmarksScreen(itemID: "movie"))),
                               ("sync-failed", AnyView(ProgressSyncScreen()))] {
            try await render(screen, name: name, scene: scene, session: disconnected)
        }
    }

    private static func render(_ screen: AnyView, name: String, scene: UIWindowScene, session: AppSession, wait: Double = 0.7) async throws {
        let previous = scene.windows.first { $0.isKeyWindow }
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: NavigationStack { screen }
            .environment(session).environment(\.scenePhase, .active)
            .tint(KinoTheme.signal).preferredColorScheme(.dark))
        window.makeKeyAndVisible()
        defer { window.isHidden = true; previous?.makeKey() }
        try await Task.sleep(for: .seconds(wait))
        if name == "reader" {
            let deadline = Date().addingTimeInterval(10)
            while Date() < deadline {
                if let web = descendants(window).compactMap({ $0 as? WKWebView }).first, web.url != nil, !web.isLoading { break }
                try await Task.sleep(for: .milliseconds(50))
            }
            guard let web = descendants(window).compactMap({ $0 as? WKWebView }).first, web.url != nil, !web.isLoading else { throw ClientError.unavailable }
            try await Task.sleep(for: .milliseconds(300))
        }
        window.layoutIfNeeded()
        // UIKit hierarchy capture omits the reader's out-of-process web layer.
        let web = name == "reader" ? descendants(window).compactMap { $0 as? WKWebView }.first : nil
        let chapter: UIImage?
        if let web {
            chapter = try await withCheckedThrowingContinuation { continuation in
                web.takeSnapshot(with: nil) { image, error in
                    if let image { continuation.resume(returning: image) }
                    else { continuation.resume(throwing: error ?? ClientError.invalidResponse) }
                }
            }
        } else { chapter = nil }
        let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in
            window.drawHierarchy(in: window.bounds, afterScreenUpdates: true)
            if let web, let chapter { chapter.draw(in: web.convert(web.bounds, to: window)) }
        }
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("native-polish-evidence")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        guard let data = image.pngData() else { throw ClientError.invalidResponse }
        try data.write(to: directory.appendingPathComponent(name + ".png"))
        print("NATIVE_POLISH_RENDER \(name) \(Int(window.bounds.width))x\(Int(window.bounds.height)) \(directory.path)")
    }

    private static func descendants(_ view: UIView) -> [UIView] { [view] + view.subviews.flatMap(descendants) }

    private static func cover(wide: Bool = false) -> Data {
        let size = wide ? CGSize(width: 900, height: 506) : CGSize(width: 600, height: 900)
        return UIGraphicsImageRenderer(size: size).pngData { context in
            UIColor(red: 0.08, green: 0.19, blue: 0.23, alpha: 1).setFill()
            context.fill(CGRect(origin: .zero, size: size))
            UIColor(red: 0.31, green: 0.57, blue: 0.60, alpha: 1).setFill()
            context.cgContext.fillEllipse(in: CGRect(x: wide ? 470 : 70, y: wide ? 70 : 130, width: 460, height: 460))
            UIColor(red: 0.06, green: 0.12, blue: 0.16, alpha: 1).setFill()
            context.fill(CGRect(x: 0, y: wide ? 340 : 470, width: size.width, height: 430))
            let attributes: [NSAttributedString.Key: Any] = [.font: UIFont.systemFont(ofSize: 34, weight: .semibold), .foregroundColor: UIColor.white]
            ("THE LAST\nOBSERVATORY" as NSString).draw(in: CGRect(x: 44, y: wide ? 370 : 640, width: 510, height: 130), withAttributes: attributes)
        }
    }
}
#endif
