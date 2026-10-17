#if os(iOS) || os(tvOS)
import SwiftUI
import Testing
import UIKit
@testable import KinosailPlayer

@Suite(.serialized) @MainActor
struct LibraryScanRetryJourneys {
    @Test func libraryScreenLoadsOnItsOwnWhenTheScanFinishes() async throws {
        try await journey("library") { _ in LibraryScreen() }
    }

    @Test func resourceViewLoadsOnItsOwnWhenTheScanFinishes() async throws {
        try await journey("resource") { session in
            ResourceView(identity: "library-scan-retry", load: { policy in
                guard let client = session.client else { throw ClientError.http(401) }
                return try await client.library(policy: policy)
            }) { page in Text("\(page.items.count) titles") }
        }
    }

    private func journey<V: View>(_ name: String, _ screen: (AppSession) -> V) async throws {
        let fixture = try await LibraryRecoveryFixture()
        defer { fixture.close() }
        fixture.libraryDeferrals = 2
        let keychain = SessionKeychain()
        let previous = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "disposable-fixture-token", viewer: fixture.viewer))
        let session = AppSession()
        await session.restore()
        do {
            let scene = try #require(UIApplication.shared.connectedScenes.first as? UIWindowScene)
            let window = UIWindow(windowScene: scene)
            let root = screen(session)
            window.rootViewController = UIHostingController(rootView: NavigationStack { root }
                .environment(session).environment(\.scenePhase, .active))
            window.isHidden = false
            defer { window.isHidden = true; window.rootViewController = nil }
            try await until("The first deferred Library read must arrive") { fixture.libraryReads >= 1 }
            try snapshot(window, name: "\(name)-scanning")
            try await until("The screen must read the Library again without a tap", seconds: 8) { fixture.libraryReads == 3 }
            try await Task.sleep(for: .seconds(2.5))
            try snapshot(window, name: "\(name)-loaded")
            let arrivals = fixture.libraryArrivals
            #expect(arrivals.count == 3, "The screen must stop reading once the Library loads")
            for (earlier, later) in zip(arrivals, arrivals.dropFirst()) {
                #expect(earlier.duration(to: later) >= .milliseconds(900), "Each read must wait for Retry-After")
            }
            await session.client?.close(purgeCache: true)
        } catch {
            await session.client?.close(purgeCache: true)
            if let previous { try await keychain.save(previous) } else { try await keychain.clear() }
            throw error
        }
        if let previous { try await keychain.save(previous) } else { try await keychain.clear() }
    }

    private func until(_ reason: String, seconds: Double = 5, _ condition: () -> Bool) async throws {
        let deadline = Date().addingTimeInterval(seconds)
        while !condition() && Date() < deadline { try await Task.sleep(for: .milliseconds(20)) }
        try #require(condition(), Comment(rawValue: reason))
    }

    private func snapshot(_ window: UIWindow, name: String) throws {
        window.layoutIfNeeded()
        let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in window.drawHierarchy(in: window.bounds, afterScreenUpdates: true) }
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("apple-library-scan-retry-evidence")
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try #require(image.pngData()).write(to: directory.appendingPathComponent(name + ".png"))
        print("APPLE_LIBRARY_SCAN_RETRY_ARTIFACT \(directory.path)")
    }
}
#endif
