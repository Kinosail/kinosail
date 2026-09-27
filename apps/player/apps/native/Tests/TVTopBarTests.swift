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

@Suite(.serialized) struct TVTopBarTests {
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
