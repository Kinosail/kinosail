#if os(tvOS)
import SwiftUI
import Testing
import UIKit
@testable import KinosailPlayer

@MainActor @Observable private final class HomeActionsProbe {
    var requestedFocus = PlayerTab.search
    var focused: PlayerTab?
    var opened: PlayerTab?
    var showsContent = false
}

private struct HomeActionsProbeScreen: View {
    @Bindable var probe: HomeActionsProbe
    @FocusState private var focus: PlayerTab?

    var body: some View {
        VStack {
            TVTopBar(focus: $focus) { probe.opened = $0 }
            if probe.showsContent { Button("Play") {} }
        }
        .onAppear { focus = .search }
        .onChange(of: probe.requestedFocus) { _, value in focus = value }
        .onChange(of: focus) { _, value in probe.focused = value }
    }
}

private struct HeaderBackgroundProbeScreen: View {
    @FocusState private var focus: PlayerTab?

    var body: some View {
        VStack(spacing: 0) {
            TVTopBar(focus: $focus) { _ in }
            Spacer()
        }
        .background(Color(red: 1, green: 0, blue: 1))
    }
}

@Suite(.serialized) struct TVTopBarTests {
    @Test @MainActor func topControlsShareThePageBackground() throws {
        let renderer = ImageRenderer(content: HeaderBackgroundProbeScreen().frame(width: 800, height: 76)
            .environment(\.colorScheme, .dark))
        renderer.scale = 1
        let image = try #require(renderer.uiImage)
        let sample = CGRect(x: 400, y: 38, width: 1, height: 1)
        let pixel = try #require(image.cgImage?.cropping(to: sample))
        var rgba = [UInt8](repeating: 0, count: 4)
        rgba.withUnsafeMutableBytes { bytes in
            let context = CGContext(data: bytes.baseAddress, width: 1, height: 1, bitsPerComponent: 8,
                                    bytesPerRow: 4, space: CGColorSpaceCreateDeviceRGB(),
                                    bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue)!
            context.draw(pixel, in: CGRect(x: 0, y: 0, width: 1, height: 1))
        }
        #expect(rgba[0] > 240 && rgba[1] < 10 && rgba[2] > 240)
    }

    @Test @MainActor func homeKeepsSearchAndSettingsWithoutLibraryMenu() async throws {
        let scene = try #require(UIApplication.shared.connectedScenes.first as? UIWindowScene)
        let probe = HomeActionsProbe()
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: HomeActionsProbeScreen(probe: probe))
        window.makeKeyAndVisible()
        defer { window.isHidden = true }

        try await Task.sleep(for: .milliseconds(300))
        #expect(probe.focused == .search)
        probe.showsContent = true
        try await Task.sleep(for: .milliseconds(100))
        #expect(probe.focused == .search)

        probe.requestedFocus = .library
        try await Task.sleep(for: .milliseconds(100))
        #expect(probe.focused != .library)
        #expect(probe.opened == nil)

        probe.requestedFocus = .settings
        try await Task.sleep(for: .milliseconds(100))
        #expect(probe.focused == .settings)
        #expect(probe.opened == nil)
    }
}
#endif
