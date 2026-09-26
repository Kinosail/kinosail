#if os(tvOS)
import SwiftUI
import Testing
import UIKit
@testable import KinosailPlayer

@MainActor @Observable private final class FocusProbe {
    var showsPlay = false
    var focused: FocusTarget?
}

private enum FocusTarget: Hashable {
    case tab, play
}

private struct FocusProbeScreen: View {
    @Bindable var probe: FocusProbe
    @FocusState private var focus: FocusTarget?
    @Namespace private var contentFocus

    var body: some View {
        VStack {
            Button("Home") {}
                .focused($focus, equals: .tab)
            if probe.showsPlay {
                Button("Play") {}
                    .focused($focus, equals: .play)
                    .tvOSDefaultPlayFocus(in: contentFocus)
            }
        }
        .focusScope(contentFocus)
        .onAppear { focus = .tab }
        .onChange(of: focus) { _, value in probe.focused = value }
    }
}

@Suite(.serialized) struct TVOSFocusTests {
    @Test @MainActor func loadedPlayDoesNotTakeFocusFromNavigation() async throws {
        let scene = try #require(UIApplication.shared.connectedScenes.first as? UIWindowScene)
        let probe = FocusProbe()
        let window = UIWindow(windowScene: scene)
        window.rootViewController = UIHostingController(rootView: FocusProbeScreen(probe: probe))
        window.makeKeyAndVisible()
        defer { window.isHidden = true }

        try await Task.sleep(for: .milliseconds(100))
        #expect(probe.focused == .tab)
        probe.showsPlay = true
        try await Task.sleep(for: .milliseconds(300))
        #expect(probe.focused == .tab)
    }
}
#endif
