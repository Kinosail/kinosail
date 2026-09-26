#if os(tvOS)
import SwiftUI
import Testing
import UIKit
@testable import KinosailPlayer

@MainActor @Observable private final class TopBarProbe {
    var selection = PlayerTab.home
    var requestedFocus = PlayerTab.home
    var focused: PlayerTab?
    var showsContent = false
}

private struct TopBarProbeScreen: View {
    @Bindable var probe: TopBarProbe
    @FocusState private var focus: PlayerTab?

    var body: some View {
        VStack {
            TVTopBar(selection: $probe.selection, focus: $focus)
            TabView(selection: $probe.selection) {
                ForEach(PlayerTab.tvPrimary) { tab in
                    Tab(tab.title, systemImage: tab.symbol, value: tab) {
                        VStack {
                            if probe.showsContent { Button("Play") {} }
                        }
                        .toolbar(.hidden, for: .tabBar)
                    }
                }
            }
        }
        .onAppear { focus = .home }
        .onChange(of: probe.requestedFocus) { _, value in focus = value }
        .onChange(of: focus) { _, value in probe.focused = value }
    }
}

@Suite(.serialized) struct TVTopBarTests {
    @Test @MainActor func movingAcrossBrowseAndUtilityControlsKeepsTopFocus() async throws {
        let scene = try #require(UIApplication.shared.connectedScenes.first as? UIWindowScene)
        let probe = TopBarProbe()
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: TopBarProbeScreen(probe: probe))
        window.makeKeyAndVisible()
        defer { window.isHidden = true }

        try await Task.sleep(for: .milliseconds(100))
        #expect(probe.focused == .home)
        probe.requestedFocus = .movies
        try await Task.sleep(for: .milliseconds(100))
        #expect(probe.selection == .movies)
        #expect(probe.focused == .movies)

        probe.showsContent = true
        try await Task.sleep(for: .milliseconds(100))
        #expect(probe.focused == .movies)

        probe.requestedFocus = .library
        try await Task.sleep(for: .milliseconds(300))
        #expect(probe.selection == .library)
        #expect(probe.focused == .library)

        probe.requestedFocus = .search
        try await Task.sleep(for: .milliseconds(100))
        #expect(probe.selection == .search)
        #expect(probe.focused == .search)
        probe.requestedFocus = .settings
        try await Task.sleep(for: .milliseconds(100))
        #expect(probe.selection == .settings)
        #expect(probe.focused == .settings)
    }
}
#endif
