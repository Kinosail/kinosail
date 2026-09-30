#if os(iOS)
import Network
import Observation
import SwiftUI
import Synchronization
import Testing
import UIKit
@testable import KinosailPlayer

@Suite(.serialized) @MainActor
struct NativePreferencesUXTests {
    @Test func cachedHeroProgressIsPresentInTheFirstRenderedFrame() async throws {
        let fixture = try await PreferencesLoopbackFixture()
        defer { fixture.listener.cancel() }
        let keychain = SessionKeychain()
        let previous = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "isolated-test-token", viewer: fixture.viewer))
        let session = AppSession()
        await session.restore()
        do {
            let item = try MediaItem(.object(["id": .string("movie"), "kind": .string("video"), "title": .string("Saved movie"),
                "artwork": .string("/art/movie"), "progress": .object(["seconds": .number(120)])]), server: fixture.server)
            let client = try #require(session.client)
            let clientID = await client.identity
            let window = try host(WatchPosition(item: item, barOnly: true), session: session)
            defer { window.isHidden = true }
            try await until { session.resourceSnapshots.value(for: "watch-progress:movie", clientID: clientID, as: WatchProgressSummary.self) != nil }
            let renderer = ImageRenderer(content: CinemaHero(item: item, showsPlot: false) { EmptyView() }
                .frame(width: 335).environment(session).environment(\.colorScheme, .dark))
            let image = try #require(renderer.cgImage)
            var pixels = [UInt8](repeating: 0, count: image.width * image.height * 4)
            pixels.withUnsafeMutableBytes { bytes in
                let context = CGContext(data: bytes.baseAddress, width: image.width, height: image.height, bitsPerComponent: 8,
                    bytesPerRow: image.width * 4, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
                context.draw(image, in: CGRect(x: 0, y: 0, width: image.width, height: image.height))
            }
            let greenPixels = stride(from: 0, to: pixels.count, by: 4).filter { pixels[$0] > 140 && pixels[$0 + 1] > 180 && pixels[$0 + 2] < 130 }.count
            #expect(greenPixels > 40)
            window.isHidden = true
            session.resourceSnapshots.remove(for: "watch-progress:movie", clientID: clientID, as: WatchProgressSummary.self)
            let model = ProgressCardModel(item: item)
            let card = try host(ProgressCardProbe(model: model), session: session)
            defer { card.isHidden = true }
            try await until { session.resourceSnapshots.isFresh(for: "watch-progress:movie", clientID: clientID, as: WatchProgressSummary.self, refreshID: session.contentRevision.uuidString) }
            let before = greenPixelsIn(card)
            #expect(before > 40)
            model.item.progress.seconds = 300
            try await Task.sleep(for: .milliseconds(100))
            #expect(greenPixelsIn(card) > before * 2, "An updated saved position must move the existing bar immediately")
            card.isHidden = true
            try await NativePolishGallery.record(session: session, fixture: fixture)
        } catch {
            try await restore(previous, keychain: keychain, session: session)
            throw error
        }
        try await restore(previous, keychain: keychain, session: session)
    }

    @Test func changingADownloadToggleSavesWithoutAnotherButton() async throws {
        let fixture = try await PreferencesLoopbackFixture()
        defer { fixture.listener.cancel() }
        let keychain = SessionKeychain()
        let previous = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "isolated-test-token", viewer: fixture.viewer))
        let session = AppSession()
        await session.restore()
        do {
            let window = try host(OfflinePreferencesScreen(), session: session)
            defer { window.isHidden = true }
            try await until { !switches(window).isEmpty }
            let control = try #require(switches(window).first)
            control.setOn(!control.isOn, animated: false)
            control.sendActions(for: .valueChanged)
            try await until("Download toggle must save without a Save button") { fixture.saves > 0 }
            #expect(fixture.saves == 1)
            #expect(fixture.preferences.wifiOnly == false)
            try snapshot(window, name: "download-preferences")
            let reads = fixture.preferenceReads
            window.isHidden = true
            let reopened = try host(OfflinePreferencesScreen(), session: session)
            defer { reopened.isHidden = true }
            try await until { !switches(reopened).isEmpty }
            #expect(switches(reopened).first?.isOn == false)
            #expect(fixture.preferenceReads == reads)
            let next = try #require(switches(reopened).last)
            next.setOn(!next.isOn, animated: false)
            next.sendActions(for: .valueChanged)
            let wifi = try #require(switches(reopened).first)
            wifi.setOn(true, animated: false)
            wifi.sendActions(for: .valueChanged)
            try await until("Rapid edits must both reach the Server") { fixture.preferences.wifiOnly && fixture.preferences.removeWatched }
        } catch {
            try await restore(previous, keychain: keychain, session: session)
            throw error
        }
        try await restore(previous, keychain: keychain, session: session)
    }

    @Test(arguments: [ScenePhase.active, .inactive, .background])
    func visibleLibraryCompletesLoadingAcrossScenePhases(phase: ScenePhase) async throws {
        let fixture = try await PreferencesLoopbackFixture()
        fixture.populated = true
        defer { fixture.listener.cancel() }
        let keychain = SessionKeychain()
        let previous = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "isolated-test-token", viewer: fixture.viewer))
        let session = AppSession()
        await session.restore()
        do {
            let client = try #require(session.client)
            let clientID = await client.identity
            let window = try host(LibraryScreen(initialView: .shows).environment(\.scenePhase, phase), session: session)
            defer { window.isHidden = true }
            try await until("An appearing library must start its request, even during a scene transition") { fixture.libraryReads > 0 }
            try await until("The loaded episode card must replace the placeholder") {
                session.resourceSnapshots.value(for: "watch-progress:episode", clientID: clientID, as: WatchProgressSummary.self) != nil
            }
            let saved: LibrarySnapshot? = session.resourceSnapshots.value(for: LibrarySnapshot.key(view: .shows), clientID: clientID)
            #expect(saved?.page.total == 1)
            #expect(saved?.items.first?.id == "episode")
            try snapshot(window, name: "library-\(phase)")
        } catch {
            try await restore(previous, keychain: keychain, session: session)
            throw error
        }
        try await restore(previous, keychain: keychain, session: session)
    }

    @Test(arguments: [ScenePhase.active, .inactive, .background])
    func visibleHomeCompletesLoadingAcrossScenePhases(phase: ScenePhase) async throws {
        let fixture = try await PreferencesLoopbackFixture()
        fixture.populated = true
        defer { fixture.listener.cancel() }
        let keychain = SessionKeychain()
        let previous = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "isolated-test-token", viewer: fixture.viewer))
        let session = AppSession()
        await session.restore()
        do {
            let client = try #require(session.client)
            let clientID = await client.identity
            let window = try host(HomeScreen(selectTab: { _ in }).environment(\.scenePhase, phase), session: session)
            defer { window.isHidden = true }
            try await until("An appearing Home must start its request, even during a scene transition") { fixture.libraryReads > 0 }
            let key = "\(session.profileKey ?? ""):watch"
            try await until("Home must replace its placeholder with the loaded library") {
                session.resourceSnapshots.value(for: key, clientID: clientID, as: HomeSnapshot.self) != nil
            }
            let saved = try #require(session.resourceSnapshots.value(for: key, clientID: clientID, as: HomeSnapshot.self))
            #expect(saved.recent.contains { $0.id == "movie" })
            try snapshot(window, name: "home-\(phase)")
        } catch {
            try await restore(previous, keychain: keychain, session: session)
            throw error
        }
        try await restore(previous, keychain: keychain, session: session)
    }

    @Test(arguments: [true, false])
    func firstArtworkKeepsItsPositionWhenLoadingCompletes(library: Bool) async throws {
        let fixture = try await PreferencesLoopbackFixture()
        fixture.populated = true; fixture.delay = 1
        let art = UIGraphicsImageRenderer(size: CGSize(width: 100, height: 150)).pngData { context in
            UIColor.magenta.setFill(); context.fill(CGRect(x: 0, y: 0, width: 100, height: 150))
        }
        fixture.artwork = art
        fixture.landscape = UIGraphicsImageRenderer(size: CGSize(width: 900, height: 506)).pngData { context in
            UIColor.magenta.setFill(); context.fill(CGRect(x: 0, y: 0, width: 900, height: 506))
        }
        defer { fixture.listener.cancel() }
        let keychain = SessionKeychain()
        let previous = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "isolated-test-token", viewer: fixture.viewer))
        let session = AppSession()
        await session.restore()
        do {
            let screen = library ? AnyView(LibraryScreen(initialView: .shows)) : AnyView(HomeScreen(selectTab: { _ in }))
            let window = try host(screen, session: session)
            defer { window.isHidden = true }
            try await until { fixture.libraryReads > 0 }
            try snapshot(window, name: "position-\(library ? "library" : "home")-pending")
            let pendingY = try #require(artworkTop(in: window, loaded: false))
            try await Task.sleep(for: .seconds(2))
            try snapshot(window, name: "position-\(library ? "library" : "home")-loaded")
            try await until { artworkTop(in: window, loaded: true) != nil }
            let loadedY = try #require(artworkTop(in: window, loaded: true))
            print("NATIVE_ARTWORK_POSITIONS library=\(library) pending=\(pendingY) loaded=\(loadedY)")
            #expect(abs(loadedY - pendingY) <= 2, "The first artwork must stay in place when real content replaces its skeleton")
        } catch {
            try await restore(previous, keychain: keychain, session: session)
            throw error
        }
        try await restore(previous, keychain: keychain, session: session)
    }

    private func artworkTop(in window: UIWindow, loaded: Bool) -> Double? {
        window.layoutIfNeeded()
        let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in window.drawHierarchy(in: window.bounds, afterScreenUpdates: true) }.cgImage!
        var pixels = [UInt8](repeating: 0, count: image.width * image.height * 4)
        pixels.withUnsafeMutableBytes { bytes in
            CGContext(data: bytes.baseAddress, width: image.width, height: image.height, bitsPerComponent: 8,
                      bytesPerRow: image.width * 4, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
                .draw(image, in: CGRect(x: 0, y: 0, width: image.width, height: image.height))
        }
        let scale = Double(image.width) / window.bounds.width
        for y in Int(140 * scale)..<image.height {
            var matches = 0
            for x in 0..<image.width {
                let index = (y * image.width + x) * 4
                let r = Int(pixels[index]), g = Int(pixels[index + 1]), b = Int(pixels[index + 2])
                if loaded ? (r > 170 && g < 60 && b > 170) : (r == 21 && g == 25 && b == 20) { matches += 1 }
            }
            if matches > image.width / 30 { return Double(y) / scale }
        }
        return nil
    }

    private func restore(_ previous: SavedSession?, keychain: SessionKeychain, session: AppSession) async throws {
        await session.client?.close()
        if let previous { try await keychain.save(previous) } else { try await keychain.clear() }
    }
    private func host<V: View>(_ view: V, session: AppSession) throws -> UIWindow {
        let scene = try #require(UIApplication.shared.connectedScenes.first as? UIWindowScene)
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: NavigationStack { view }
            .environment(session).environment(\.scenePhase, .active).preferredColorScheme(.dark))
        window.isHidden = false
        window.layoutIfNeeded()
        return window
    }
    private func switches(_ view: UIView) -> [UISwitch] {
        (view as? UISwitch).map { [$0] } ?? view.subviews.flatMap(switches)
    }
    private func until(_ reason: String = "The expected native view must appear", _ condition: () -> Bool) async throws {
        let deadline = Date().addingTimeInterval(5)
        while !condition() && Date() < deadline { try await Task.sleep(for: .milliseconds(20)) }
        try #require(condition(), Comment(rawValue: reason))
    }
    private func snapshot(_ window: UIWindow, name: String) throws {
        let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in window.drawHierarchy(in: window.bounds, afterScreenUpdates: true) }
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("native-polish-evidence")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try #require(image.pngData()).write(to: directory.appendingPathComponent(name + ".png"))
        print("NATIVE_POLISH_ARTIFACT \(directory.path)")
    }
    private func greenPixelsIn(_ window: UIWindow) -> Int {
        window.layoutIfNeeded()
        let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in window.drawHierarchy(in: window.bounds, afterScreenUpdates: true) }.cgImage!
        var pixels = [UInt8](repeating: 0, count: image.width * image.height * 4)
        pixels.withUnsafeMutableBytes { bytes in
            CGContext(data: bytes.baseAddress, width: image.width, height: image.height, bitsPerComponent: 8,
                      bytesPerRow: image.width * 4, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
                .draw(image, in: CGRect(x: 0, y: 0, width: image.width, height: image.height))
        }
        return stride(from: 0, to: pixels.count, by: 4).filter { pixels[$0] > 140 && pixels[$0 + 1] > 180 && pixels[$0 + 2] < 130 }.count
    }
}

@MainActor @Observable private final class ProgressCardModel {
    var item: MediaItem
    init(item: MediaItem) { self.item = item }
}
private struct ProgressCardProbe: View {
    let model: ProgressCardModel
    var body: some View { WatchPosition(item: model.item, barOnly: true).frame(width: 300) }
}
#endif
