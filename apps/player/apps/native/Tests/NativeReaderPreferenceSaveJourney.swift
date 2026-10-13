#if os(iOS)
import Testing
import UIKit
import WebKit
@testable import KinosailPlayer

// A pending reader and another native consumer share the real preference request.
// Browser E2E cannot host this native shared-client cancellation boundary.
extension NativePreferencesUXTests {
    @Test func readerPreferenceSaveDoesNotAbandonOpeningChapter() async throws {
        let fixture = try await PreferencesLoopbackFixture()
        defer { fixture.releaseReader(); fixture.releasePreferences(); fixture.listener.cancel() }
        let keychain = SessionKeychain(), previous = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "isolated-reader-save-token", viewer: fixture.viewer))
        let session = AppSession()
        await session.restore()
        do {
            let scene = try #require(UIApplication.shared.connectedScenes.first as? UIWindowScene)
            let previousWindow = scene.windows.first { $0.isKeyWindow }
            let window = UIWindow(windowScene: scene)
            defer { window.isHidden = true; previousWindow?.makeKey() }
            let folder = FileManager.default.temporaryDirectory.appendingPathComponent("native-reader-column-" + UUID().uuidString)
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
            print("NATIVE_READER_EVIDENCE \(folder.path)")
            fixture.readerHeld = true; fixture.preferencesHeld = true
            readerRoot(window, session: session, light: true)
            window.makeKeyAndVisible()
            let client = try #require(session.client)
            try await readerUntil { fixture.heldReaderCount == 1 && fixture.heldPreferenceCount == 1 }
            let survivor = Task { try await client.mediaPreferences() }
            defer { survivor.cancel() }
            try await readerCapture(window, folder: folder, name: "pending-before-save")
            var edited = MediaPreferences(); edited.readerFontSize = 32
            #expect(try await client.saveMediaPreferences(edited) == edited)
            #expect(fixture.saves == 1 && fixture.heldPreferenceCount == 1)
            fixture.releaseReader()
            try await readerLoadedWebView(window, fixture: fixture, folder: folder,
                reason: "A successful preference save must complete the active reader without releasing the obsolete GET")
            #expect(try await survivor.value == edited)
            let web = try #require(readerDescendants(window).compactMap { $0 as? WKWebView }.first)
            let metrics = try await readerMetrics(web)
            #expect(Double(metrics.fontSize.replacingOccurrences(of: "px", with: "")) == 32)
            #expect(try readerBands(window, light: true).isEmpty)
            try await readerCapture(window, folder: folder, name: "loaded-after-save")
            fixture.releasePreferences()
            #expect(try await client.mediaPreferences(policy: .cached) == edited)
            #expect(fixture.preferenceReads == 1)
            try readerEvidence(JSONSerialization.data(withJSONObject: ["savedFontSize": 32, "loadedFontSize": metrics.fontSize,
                "preferenceReads": fixture.preferenceReads, "saves": fixture.saves,
                "obsoleteResponseReleasedAfterLoaded": true], options: [.prettyPrinted, .sortedKeys]),
                folder: folder, name: "concurrent-save.json")
        } catch {
            await session.client?.close(purgeCache: true)
            if let previous { try await keychain.save(previous) } else { try await keychain.clear() }
            throw error
        }
        await session.client?.close(purgeCache: true)
        if let scope = session.profileKey { try await ReaderPositionStore(scope: scope).discard(itemID: "book") }
        if let previous { try await keychain.save(previous) } else { try await keychain.clear() }
    }
}
#endif
