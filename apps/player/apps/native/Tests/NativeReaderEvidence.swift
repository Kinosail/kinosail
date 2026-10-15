#if os(iOS)
import Testing
import UIKit
import WebKit
@testable import KinosailPlayer

extension NativePreferencesUXTests {
    func readerEvidence(_ data: Data, folder: URL, name: String) throws {
        try data.write(to: folder.appendingPathComponent(name))
        Attachment.record(data, named: name)
    }

    func readerLoadedWebView(_ window: UIWindow, fixture: PreferencesLoopbackFixture, folder: URL,
                             reason: String = "The held reader operation must reach its expected visible state") async throws {
        do {
            try await readerUntil(reason) {
                readerDescendants(window).compactMap { $0 as? WKWebView }.contains { $0.url != nil && !$0.isLoading }
            }
        } catch {
            let views = readerDescendants(window)
            let web = views.compactMap { $0 as? WKWebView }
            let now = ProcessInfo.processInfo.systemUptime
            let fixtureState = fixture.state.withLock { ($0.readerAssetReads, $0.readerEvents.mapValues { now - $0 }) }
            let diagnostic: [String: Any] = ["keyWindow": window.isKeyWindow, "hidden": window.isHidden,
                "sceneState": window.windowScene?.activationState.rawValue ?? -1,
                "protectedViews": views.filter { $0 is ProtectedReaderView }.count,
                "assetReads": fixtureState.0, "eventAgesSeconds": fixtureState.1,
                "webViews": web.prefix(4).map { ["hasURL": $0.url != nil, "loading": $0.isLoading,
                    "progress": $0.estimatedProgress, "attached": $0.window === window,
                    "width": $0.bounds.width, "height": $0.bounds.height] as [String: Any] }]
            if let data = try? JSONSerialization.data(withJSONObject: diagnostic, options: [.sortedKeys]) {
                try? readerEvidence(data, folder: folder, name: "loaded-webview-state.json")
                print("NATIVE_READER_WINDOW_FAILURE " + String(decoding: data, as: UTF8.self))
            }
            let image = UIGraphicsImageRenderer(bounds: window.bounds).image { _ in
                window.drawHierarchy(in: window.bounds, afterScreenUpdates: true)
            }
            if let data = image.pngData() { try? readerEvidence(data, folder: folder, name: "loaded-webview-timeout.png") }
            let observationStart = ProcessInfo.processInfo.systemUptime
            var transitions: [[String: Any]] = []
            var previousReads = fixture.state.withLock { $0.readerAssetReads }
            var previousLoading = web.first?.isLoading
            while ProcessInfo.processInfo.systemUptime - observationStart < 15, !Task.isCancelled {
                do { try await Task.sleep(for: .milliseconds(50)) } catch { break }
                let reads = fixture.state.withLock { $0.readerAssetReads }
                let loading = web.first?.isLoading
                if reads != previousReads || loading != previousLoading {
                    if transitions.count < 8 {
                        transitions.append(["elapsedMs": (ProcessInfo.processInfo.systemUptime - observationStart) * 1000,
                            "assetReads": reads, "loading": loading ?? false, "hasWebView": !web.isEmpty,
                            "progress": web.first?.estimatedProgress ?? 0])
                    }
                    previousReads = reads; previousLoading = loading
                }
                if let view = web.first, view.url != nil, !view.isLoading { break }
            }
            let followup: [String: Any] = ["originalFailureRetained": true,
                "elapsedMs": (ProcessInfo.processInfo.systemUptime - observationStart) * 1000,
                "assetReads": fixture.state.withLock { $0.readerAssetReads }, "transitions": transitions,
                "sameWebViewAttached": web.first?.window === window,
                "loading": web.first?.isLoading ?? false, "progress": web.first?.estimatedProgress ?? 0,
                "cancelled": Task.isCancelled]
            if let data = try? JSONSerialization.data(withJSONObject: followup, options: [.sortedKeys]) {
                try? readerEvidence(data, folder: folder, name: "loaded-webview-after-timeout.json")
                print("NATIVE_READER_AFTER_TIMEOUT " + String(decoding: data, as: UTF8.self))
            }
            throw error
        }
    }
}
#endif
