#if os(iOS)
import SwiftUI
import Testing
import UIKit
import WebKit
@testable import KinosailPlayer

// The existing PDF spinner test and loaded-only gallery cannot compare native
// pending pixels with WKWebView layout. This isolated native UI journey uses
// held loopback metadata and production screens, without a production seam.
// Keep it in the serialized preferences owner because both use SessionKeychain.
extension NativePreferencesUXTests {
    @Test(arguments: [false, true])
    func readerPendingMatchesLoadedColumnAndClearsAtTerminalStates(light: Bool) async throws {
        let fixture = try await PreferencesLoopbackFixture()
        defer { fixture.releaseReader(); fixture.listener.cancel() }
        let keychain = SessionKeychain(), previousSession = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "isolated-reader-token", viewer: fixture.viewer))
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
            fixture.readerHeld = true
            readerRoot(window, session: session, light: light)
            window.makeKeyAndVisible()
            try await readerUntil { fixture.heldReaderCount == 1 }
            let pending = try readerBands(window, light: light)
            try #require(!pending.isEmpty, "Prerequisite: real pending lines must be visible while metadata is held")
            try await readerCapture(window, folder: folder, name: "pending")
            fixture.releaseReader()
            try await readerUntil { readerDescendants(window).compactMap { $0 as? WKWebView }.contains { $0.url != nil && !$0.isLoading } }
            let web = try #require(readerDescendants(window).compactMap { $0 as? WKWebView }.first)
            let metrics = try await readerMetrics(web)
            let origin = web.convert(web.bounds, to: window)
            let left = origin.minX + metrics.left, right = origin.minX + metrics.right
            let receipt: [String: Any] = ["light": light, "windowWidth": window.bounds.width,
                "pendingLeft": pending[0].minX, "pendingRight": pending[0].maxX,
                "loadedLeft": left, "loadedRight": right, "fontFamily": metrics.fontFamily,
                "fontSize": metrics.fontSize, "horizontalOverflow": metrics.overflow]
            try JSONSerialization.data(withJSONObject: receipt, options: [.prettyPrinted, .sortedKeys])
                .write(to: folder.appendingPathComponent("geometry.json"))
            try await readerCapture(window, folder: folder, name: "loaded")
            #expect(abs(pending[0].minX - left) <= 2, "Pending and loaded column left edges must match")
            #expect(abs(pending[0].maxX - right) <= 2, "Pending and loaded column right edges must match")
            #expect(metrics.overflow <= 1)
            #expect(try readerBands(window, light: light).isEmpty, "Success must remove the pending lines")
            for state in ["empty", "failed"] {
                fixture.readerEmpty = state == "empty"; fixture.readerFails = state == "failed"
                fixture.readerHeld = true
                readerRoot(window, session: session, light: light)
                try await readerUntil { fixture.heldReaderCount == 1 }
                #expect(try !readerBands(window, light: light).isEmpty)
                fixture.releaseReader()
                try await readerUntil {
                    readerDescendants(window).compactMap { $0 as? WKWebView }.isEmpty &&
                        (try? readerBands(window, light: light).isEmpty) == true
                }
                try await readerCapture(window, folder: folder, name: state)
            }
        } catch {
            await session.client?.close(purgeCache: true)
            if let previousSession { try await keychain.save(previousSession) } else { try await keychain.clear() }
            throw error
        }
        await session.client?.close(purgeCache: true)
        if let scope = session.profileKey { try await ReaderPositionStore(scope: scope).discard(itemID: "book") }
        if let previousSession { try await keychain.save(previousSession) } else { try await keychain.clear() }
    }

    private func readerRoot(_ window: UIWindow, session: AppSession, light: Bool) {
        window.overrideUserInterfaceStyle = light ? .light : .dark
        window.rootViewController = UIHostingController(rootView: NavigationStack { ReaderScreen(itemID: "book") }
            .environment(session).environment(\.scenePhase, .active)
            .transaction { $0.animation = nil; $0.disablesAnimations = true }
            .preferredColorScheme(light ? .light : .dark))
        window.layoutIfNeeded()
    }

    private func readerUntil(_ condition: () -> Bool) async throws {
        let deadline = ContinuousClock.now.advanced(by: .seconds(10))
        while !condition() && ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(20)) }
        try #require(condition(), "The held reader operation must reach its expected visible state")
    }

    private func readerDescendants(_ view: UIView) -> [UIView] { [view] + view.subviews.flatMap(readerDescendants) }

    private func readerBands(_ window: UIWindow, light: Bool) throws -> [CGRect] {
        window.layoutIfNeeded()
        let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in
            window.drawHierarchy(in: window.bounds, afterScreenUpdates: true)
        }
        let cg = try #require(image.cgImage), width = cg.width, height = cg.height
        var pixels = [UInt8](repeating: 0, count: width * height * 4)
        pixels.withUnsafeMutableBytes { data in
            CGContext(data: data.baseAddress, width: width, height: height, bitsPerComponent: 8,
                bytesPerRow: width * 4, space: CGColorSpaceCreateDeviceRGB(),
                bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!.draw(cg, in: CGRect(x: 0, y: 0, width: width, height: height))
        }
        let color = UIColor(KinoTheme.surface).resolvedColor(with: UITraitCollection(userInterfaceStyle: light ? .light : .dark))
        var red: CGFloat = 0, green: CGFloat = 0, blue: CGFloat = 0, alpha: CGFloat = 0
        color.getRed(&red, green: &green, blue: &blue, alpha: &alpha)
        let target = [Int((red * 255).rounded()), Int((green * 255).rounded()), Int((blue * 255).rounded())]
        let scale = CGFloat(width) / window.bounds.width
        var groups: [CGRect] = [], current: CGRect?
        for y in 0..<height {
            var first: Int?, last = 0, matches = 0
            for x in 0..<width {
                let offset = (y * width + x) * 4
                if abs(Int(pixels[offset]) - target[0]) <= 2 &&
                    abs(Int(pixels[offset + 1]) - target[1]) <= 2 && abs(Int(pixels[offset + 2]) - target[2]) <= 2 {
                    if first == nil { first = x }; last = x; matches += 1
                }
            }
            if let first, matches > width / 8 {
                let row = CGRect(x: CGFloat(first) / scale, y: CGFloat(y) / scale,
                    width: CGFloat(last - first + 1) / scale, height: 1 / scale)
                current = current.map { $0.union(row) } ?? row
            } else if let group = current {
                // This fixture uses 20px body text. Exclude native navigation
                // and footer surfaces when locating its paragraph placeholders.
                if (8...40).contains(group.height) { groups.append(group) }
                current = nil
            }
        }
        return groups.sorted { $0.width > $1.width }
    }

    private func readerMetrics(_ web: WKWebView) async throws -> (left: Double, right: Double, fontSize: String, fontFamily: String, overflow: Double) {
        let script = """
        JSON.stringify((()=>{const b=document.body,r=b.getBoundingClientRect(),s=getComputedStyle(b);return {
        left:r.left+parseFloat(s.paddingLeft),right:r.right-parseFloat(s.paddingRight),fontSize:s.fontSize,
        fontFamily:s.fontFamily,overflow:document.documentElement.scrollWidth-innerWidth}})())
        """
        let text: String = try await withCheckedThrowingContinuation { continuation in
            web.evaluateJavaScript(script) { value, error in
                if let error { continuation.resume(throwing: error) }
                else if let text = value as? String { continuation.resume(returning: text) }
                else { continuation.resume(throwing: ClientError.invalidResponse) }
            }
        }
        let value = try #require(JSONSerialization.jsonObject(with: Data(text.utf8)) as? [String: Any])
        return (try #require(value["left"] as? Double), try #require(value["right"] as? Double),
                try #require(value["fontSize"] as? String), try #require(value["fontFamily"] as? String),
                try #require(value["overflow"] as? Double))
    }

    private func readerCapture(_ window: UIWindow, folder: URL, name: String) async throws {
        let web = readerDescendants(window).compactMap { $0 as? WKWebView }.first
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
        try #require(image.pngData()).write(to: folder.appendingPathComponent(name + ".png"))
    }
}
#endif
