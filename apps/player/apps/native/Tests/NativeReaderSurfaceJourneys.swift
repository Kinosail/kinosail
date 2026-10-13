#if os(iOS)
import SwiftUI
import Testing
import UIKit
import Vision
import WebKit
@testable import KinosailPlayer

// The existing PDF spinner test and loaded-only gallery cannot compare native
// pending pixels with WKWebView layout. This isolated native UI journey uses
// held loopback metadata and production screens, without a production seam.
// Keep it in the serialized preferences owner because both use SessionKeychain.
extension NativePreferencesUXTests {
    @Test(arguments: [("en", "appearance", "chapter2of3", "page2of3", "document"),
                      ("es", "apariencia", "capitulo2de3", "pagina2de3", "documento"),
                      ("ja", "外観", "第2章全3章", "2ページ全3ページ", "文書")],
          [ReaderBook.Kind.epub, .comic, .pdf])
    func readerFooterUsesSelectedInterfaceLocale(language: (tag: String, control: String, chapter: String, page: String, document: String), kind: ReaderBook.Kind) async throws {
        let fixture = try await PreferencesLoopbackFixture()
        defer { fixture.listener.cancel() }
        let asset = kind == .pdf ? UIGraphicsPDFRenderer(bounds: CGRect(x: 0, y: 0, width: 400, height: 600)).pdfData { context in
            for _ in 0..<3 { context.beginPage(); ("A quiet morning" as NSString).draw(at: CGPoint(x: 20, y: 40), withAttributes: [.font: UIFont.systemFont(ofSize: 20)]) }
        } : kind == .comic ? UIGraphicsImageRenderer(size: CGSize(width: 400, height: 600)).pngData { context in
            UIColor.systemBlue.setFill(); context.fill(CGRect(x: 0, y: 0, width: 400, height: 600))
        } : Data()
        fixture.state.withLock { $0.readerKind = kind; $0.readerPageCount = kind == .pdf ? 1 : 3; $0.readerPage = kind == .pdf ? 1 : 2; $0.readerAsset = asset }
        let keychain = SessionKeychain(), previous = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "isolated-reader-token", viewer: fixture.viewer))
        let session = AppSession()
        await session.restore()
        do {
            let scene = try #require(UIApplication.shared.connectedScenes.first as? UIWindowScene)
            let previousWindow = scene.windows.first { $0.isKeyWindow }
            let window = UIWindow(windowScene: scene)
            defer { window.isHidden = true; previousWindow?.makeKey() }
            window.overrideUserInterfaceStyle = .dark
            // This explicit harness control proves the selected catalog is active.
            // It is outside the real reader and is not part of the product UI.
            window.rootViewController = UIHostingController(rootView: VStack(spacing: 0) {
                NavigationStack { ReaderScreen(itemID: "book") }
                Text("Appearance").padding(8)
            }.environment(session).environment(\.scenePhase, .active)
                .environment(\.locale, Locale(identifier: language.tag))
                .preferredColorScheme(.dark))
            let folder = FileManager.default.temporaryDirectory.appendingPathComponent("native-reader-column-" + UUID().uuidString)
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
            print("NATIVE_READER_EVIDENCE \(folder.path)")
            let shown = ProcessInfo.processInfo.systemUptime
            window.makeKeyAndVisible()
            try await readerUntil("Validated metadata must present the native reading footer") {
                readerDescendants(window).contains { $0 is ProtectedReaderView }
            }
            window.layoutIfNeeded()
            // The native footer is available before document decoding completes.
            // Capture its pixels without coupling localization to WebKit readiness.
            let footer = UIGraphicsImageRenderer(size: CGSize(width: window.bounds.width, height: 180)).image { context in
                context.cgContext.translateBy(x: 0, y: 180 - window.bounds.height)
                window.drawHierarchy(in: window.bounds, afterScreenUpdates: true)
            }
            try readerEvidence(#require(footer.pngData()), folder: folder, name: "localized-footer.png")
            let image = try #require(footer.cgImage)
            let request = VNRecognizeTextRequest()
            request.recognitionLevel = .accurate
            request.recognitionLanguages = language.tag == "en" ? ["en"] : [language.tag, "en"]
            request.usesLanguageCorrection = false
            try VNImageRequestHandler(cgImage: image).perform([request])
            let text = (request.results ?? []).compactMap { $0.topCandidates(1).first?.string }.joined(separator: " ")
            let normalized = String(String.UnicodeScalarView(text.folding(options: [.diacriticInsensitive, .caseInsensitive], locale: Locale(identifier: language.tag))
                .unicodeScalars.filter { CharacterSet.alphanumerics.contains($0) }))
            let expected = kind == .pdf ? language.document : kind == .epub ? language.chapter : language.page
            let evidence: [String: Any] = ["locale": language.tag, "kind": kind.rawValue, "recognizedText": text,
                "proofClass": "metadata-ready native footer",
                "secondsSinceShown": ProcessInfo.processInfo.systemUptime - shown,
                "requestEvents": fixture.state.withLock { $0.readerEvents.mapValues { $0 - shown } },
                "positiveControlTranslated": normalized.contains(language.control),
                "expectedCounterMatched": normalized.contains(expected)]
            try readerEvidence(JSONSerialization.data(withJSONObject: evidence, options: [.prettyPrinted, .sortedKeys]),
                folder: folder, name: "localization.json")
            #expect(normalized.contains(language.control), "The explicit harness control must confirm the selected catalog")
            #expect(normalized.contains(expected), "The real reader must localize the counter and preserve unequal page numbers")
        } catch {
            await session.client?.close(purgeCache: true)
            if let previous { try await keychain.save(previous) } else { try await keychain.clear() }
            throw error
        }
        await session.client?.close(purgeCache: true)
        if let scope = session.profileKey { try await ReaderPositionStore(scope: scope).discard(itemID: "book") }
        if let previous { try await keychain.save(previous) } else { try await keychain.clear() }
    }

    @Test(arguments: [false, true], [20, 32])
    func readerPendingMatchesLoadedColumnAndClearsAtTerminalStates(light: Bool, fontSize: Int) async throws {
        let fixture = try await PreferencesLoopbackFixture()
        defer { fixture.releaseReader(); fixture.releasePreferences(); fixture.listener.cancel() }
        fixture.state.withLock { $0.preferences.readerFontSize = fontSize }
        let keychain = SessionKeychain(), previousSession = try await keychain.restore()
        try await keychain.save(SavedSession(server: fixture.server, token: "isolated-reader-token", viewer: fixture.viewer))
        let session = AppSession()
        await session.restore()
        var evidenceFolder: URL?
        var stage = "initial"
        do {
            let scene = try #require(UIApplication.shared.connectedScenes.first as? UIWindowScene)
            let previousWindow = scene.windows.first { $0.isKeyWindow }
            let window = UIWindow(windowScene: scene)
            defer { window.isHidden = true; previousWindow?.makeKey() }
            let folder = FileManager.default.temporaryDirectory.appendingPathComponent("native-reader-column-" + UUID().uuidString)
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
            evidenceFolder = folder
            print("NATIVE_READER_EVIDENCE \(folder.path)")
            fixture.readerHeld = true
            readerRoot(window, session: session, light: light)
            window.makeKeyAndVisible()
            stage = "initial-metadata-start"
            try await readerUntil { fixture.heldReaderCount == 1 }
            stage = "initial-pending-capture"
            let initial = try readerBands(window, light: light)
            try readerEvidence(JSONSerialization.data(withJSONObject: ["requestedFontSize": fontSize,
                "observedPendingHeights": initial.map(\.height)], options: [.prettyPrinted]),
                folder: folder, name: "pending-font.json")
            try await readerCapture(window, folder: folder, name: "pending-initial")
            if fontSize != 20 {
                stage = "saved-font-pending"
                try await readerUntil("Pending lines must adopt the confirmed saved font before book metadata is released") {
                    (try? readerBands(window, light: light).contains { abs($0.height - CGFloat(fontSize)) <= 1 }) == true
                }
            }
            let pending = try readerBands(window, light: light)
            try #require(!pending.isEmpty, "Prerequisite: real pending lines must be visible while metadata is held")
            try await readerCapture(window, folder: folder, name: "pending")
            fixture.releaseReader()
            stage = "loaded-webview"
            try await readerLoadedWebView(window, fixture: fixture, folder: folder)
            stage = "loaded-geometry"
            let web = try #require(readerDescendants(window).compactMap { $0 as? WKWebView }.first)
            let metrics = try await readerMetrics(web)
            let origin = web.convert(web.bounds, to: window)
            let left = origin.minX + metrics.left, right = origin.minX + metrics.right
            let receipt: [String: Any] = ["light": light, "windowWidth": window.bounds.width,
                "pendingLeft": pending[0].minX, "pendingRight": pending[0].maxX,
                "loadedLeft": left, "loadedRight": right, "fontFamily": metrics.fontFamily,
                "fontSize": metrics.fontSize, "horizontalOverflow": metrics.overflow]
            try readerEvidence(JSONSerialization.data(withJSONObject: receipt, options: [.prettyPrinted, .sortedKeys]),
                folder: folder, name: "geometry.json")
            try await readerCapture(window, folder: folder, name: "loaded")
            #expect(abs(pending[0].minX - left) <= 2, "Pending and loaded column left edges must match")
            #expect(abs(pending[0].maxX - right) <= 2, "Pending and loaded column right edges must match")
            #expect(metrics.overflow <= 1)
            #expect(try readerBands(window, light: light).isEmpty, "Success must remove the pending lines")
            for state in ["empty", "failed"] {
                fixture.readerEmpty = state == "empty"; fixture.readerFails = state == "failed"
                fixture.preferencesHeld = state == "failed"
                fixture.readerHeld = true
                readerRoot(window, session: session, light: light)
                stage = state + "-metadata-start"
                try await readerUntil { fixture.heldReaderCount == 1 }
                stage = state + "-preference-start"
                if state == "failed" { try await readerUntil { fixture.heldPreferenceCount == 1 } }
                let client = try #require(session.client)
                let survivor: Task<MediaPreferences, Error>? = state == "failed" ? Task { try await client.mediaPreferences() } : nil
                defer { survivor?.cancel() }
                #expect(try !readerBands(window, light: light).isEmpty)
                fixture.releaseReader()
                stage = state + "-terminal"
                try await readerUntil("Metadata failure must clear pending UI while preferences are held") {
                    readerDescendants(window).compactMap { $0 as? WKWebView }.isEmpty &&
                        (try? readerBands(window, light: light).isEmpty) == true
                }
                try await readerCapture(window, folder: folder, name: state)
                fixture.releasePreferences()
                if let survivor {
                    #expect(try await survivor.value.readerFontSize == fontSize,
                            "Canceling the failed reader must preserve the shared preference producer")
                    #expect(try await client.mediaPreferences(policy: .cached).readerFontSize == fontSize,
                            "The producer must retain its successful validated result")
                }
            }
        } catch {
            if let evidenceFolder {
                let diagnostic: [String: Any] = ["stage": stage, "light": light, "fontSize": fontSize, "heldReaderCount": fixture.heldReaderCount,
                    "heldPreferenceCount": fixture.heldPreferenceCount, "preferenceReads": fixture.preferenceReads]
                if let data = try? JSONSerialization.data(withJSONObject: diagnostic, options: [.sortedKeys]) {
                    try? readerEvidence(data, folder: evidenceFolder, name: "failed-operation.json")
                    // Retain only fixed stage names and fixture counts in hosted logs.
                    print("NATIVE_READER_FAILURE " + String(decoding: data, as: UTF8.self))
                }
            }
            await session.client?.close(purgeCache: true)
            if let previousSession { try await keychain.save(previousSession) } else { try await keychain.clear() }
            throw error
        }
        await session.client?.close(purgeCache: true)
        if let scope = session.profileKey { try await ReaderPositionStore(scope: scope).discard(itemID: "book") }
        if let previousSession { try await keychain.save(previousSession) } else { try await keychain.clear() }
    }

    func readerRoot(_ window: UIWindow, session: AppSession, light: Bool) {
        window.overrideUserInterfaceStyle = light ? .light : .dark
        window.rootViewController = UIHostingController(rootView: NavigationStack { ReaderScreen(itemID: "book") }
            .environment(session).environment(\.scenePhase, .active)
            .transaction { $0.animation = nil; $0.disablesAnimations = true }
            .preferredColorScheme(light ? .light : .dark))
        window.layoutIfNeeded()
    }

    func readerUntil(_ reason: String = "The held reader operation must reach its expected visible state", _ condition: () -> Bool) async throws {
        let deadline = ContinuousClock.now.advanced(by: .seconds(10))
        while !condition() && ContinuousClock.now < deadline { try await Task.sleep(for: .milliseconds(20)) }
        try #require(condition(), Comment(rawValue: reason))
    }

    func readerDescendants(_ view: UIView) -> [UIView] { [view] + view.subviews.flatMap(readerDescendants) }

    func readerBands(_ window: UIWindow, light: Bool) throws -> [CGRect] {
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
                // This fixture uses at most 32px body text. Exclude native navigation
                // and footer surfaces when locating its paragraph placeholders.
                if (8...40).contains(group.height) { groups.append(group) }
                current = nil
            }
        }
        return groups.sorted { $0.width > $1.width }
    }

    func readerMetrics(_ web: WKWebView) async throws -> (left: Double, right: Double, fontSize: String, fontFamily: String, overflow: Double) {
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

    func readerCapture(_ window: UIWindow, folder: URL, name: String) async throws {
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
        try readerEvidence(#require(image.pngData()), folder: folder, name: name + ".png")
    }

}
#endif
