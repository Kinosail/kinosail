#if os(iOS) || os(tvOS)
import SwiftUI
import Testing
import UIKit
@testable import KinosailPlayer

/// Protects the native rendering gap in populated browser E2E coverage. The
/// production photo view and HTTP transport run against a loopback stand-in.
@Suite(.serialized) @MainActor
struct PhotoAuthorizationJourneys {
    @Test(arguments: [401, 403, 404])
    func revokedPhotoDisappearsAfterSavedContentWasShown(status: Int) async throws {
        try await journey(status: status, retainsPhoto: false)
    }

    @Test func connectionFailureKeepsTheSavedPhoto() async throws {
        try await journey(status: 503, retainsPhoto: true)
    }

    private func journey(status: Int, retainsPhoto: Bool) async throws {
        let fixture = try await LibraryRecoveryFixture()
        defer { fixture.close() }
        let keychain = SessionKeychain()
        let previous = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "disposable-fixture-token", viewer: fixture.viewer))
        let session = AppSession()
        await session.restore()
        do {
            let client = try #require(session.client)
            _ = try await client.item(id: "photo", policy: .reload)
            _ = try await session.artwork.image(path: "/media/photo", client: client, dimension: 4096)
            let cache = try await client.cacheStore()
            await cache?.flushWrites()
            await client.invalidateCatalog()
            fixture.held = true
            let scene = try #require(UIApplication.shared.connectedScenes.first as? UIWindowScene)
            let window = UIWindow(windowScene: scene)
            window.rootViewController = UIHostingController(rootView: NavigationStack { PhotoScreen(itemID: "photo") }.environment(session))
            window.isHidden = false
            defer { window.isHidden = true; window.rootViewController = nil }
            try await until("The saved photo must render while its refresh is pending") { showsPhoto(window) && fixture.photoReads == 2 }
            try snapshot(window, name: "photo-\(status)-saved")
            fixture.respondPhoto(status: status)
            if status == 503 {
                // Safe GET retries have two more requests. Release each through
                // the same HTTP boundary before assessing the settled view.
                for count in 3...4 {
                    try await until { fixture.photoReads == count }
                    fixture.respondPhoto(status: status)
                }
            }
            // Synchronization only: assertions remain on rendered content. The
            // transport producer must finish before a retained image can count
            // as successful offline recovery.
            let deadline = Date().addingTimeInterval(5)
            while !(await client.catalogRequests.isEmpty), Date() < deadline { try await Task.sleep(for: .milliseconds(20)) }
            let requestFinished = await client.catalogRequests.isEmpty
            try #require(requestFinished, "The photo refresh must finish before its UI is assessed")
            await Task.yield()
            let settled = await eventually { showsPhoto(window) == retainsPhoto }
            try snapshot(window, name: "photo-\(status)-settled")
            #expect(settled, "Explicit denial must remove cached pixels; ordinary reachability failure must preserve them")
            await client.close(purgeCache: true)
        } catch {
            await session.client?.close(purgeCache: true)
            if let previous { try await keychain.save(previous) } else { try await keychain.clear() }
            throw error
        }
        if let previous { try await keychain.save(previous) } else { try await keychain.clear() }
    }

    private func photos(_ view: UIView) -> [UIImageView] {
        if let image = view as? UIImageView, image.image != nil, image.accessibilityLabel == "Fixture photo" { return [image] }
        return view.subviews.flatMap(photos)
    }

    private func showsPhoto(_ window: UIWindow) -> Bool {
        #if os(iOS)
        return photos(window).count == 1
        #else
        window.layoutIfNeeded()
        let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in window.drawHierarchy(in: window.bounds, afterScreenUpdates: true) }.cgImage!
        var pixels = [UInt8](repeating: 0, count: image.width * image.height * 4)
        pixels.withUnsafeMutableBytes { bytes in
            CGContext(data: bytes.baseAddress, width: image.width, height: image.height, bitsPerComponent: 8,
                      bytesPerRow: image.width * 4, space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
                .draw(image, in: CGRect(x: 0, y: 0, width: image.width, height: image.height))
        }
        return stride(from: 0, to: pixels.count, by: 4).filter { pixels[$0] > 200 && pixels[$0 + 1] < 50 && pixels[$0 + 2] > 200 }.count > 1000
        #endif
    }

    private func until(_ reason: String = "The expected HTTP request must arrive", _ condition: () -> Bool) async throws {
        let ready = await eventually(condition)
        try #require(ready, Comment(rawValue: reason))
    }

    private func eventually(_ condition: () -> Bool) async -> Bool {
        let deadline = Date().addingTimeInterval(5)
        var matches = 0
        while Date() < deadline, !Task.isCancelled {
            matches = condition() ? matches + 1 : 0
            if matches == 2 { return true }
            try? await Task.sleep(for: .milliseconds(20))
        }
        return false
    }

    private func snapshot(_ window: UIWindow, name: String) throws {
        window.layoutIfNeeded()
        let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in window.drawHierarchy(in: window.bounds, afterScreenUpdates: true) }
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("apple-library-recovery-evidence")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try #require(image.pngData()).write(to: directory.appendingPathComponent(name + ".png"))
        print("APPLE_LIBRARY_RECOVERY_ARTIFACT \(directory.path)")
    }
}
#endif
