import SwiftUI

private struct TVOSDefaultPlayFocus: ViewModifier {
    let namespace: Namespace.ID
    let layoutID: String
    let enabled: Bool
    #if os(tvOS)
    @Environment(\.resetFocus) private var resetFocus
    #endif

    func body(content: Content) -> some View {
        #if os(tvOS)
        content
            .id(layoutID)
            .accessibilityIdentifier(layoutID)
            .prefersDefaultFocus(enabled, in: namespace)
            .onAppear { requestFocusIfNeeded() }
            .onChange(of: enabled) { _, isEnabled in
                if isEnabled { requestFocusIfNeeded() }
            }
        #else
        content
        #endif
    }

    #if os(tvOS)
    private func requestFocusIfNeeded() {
        guard enabled else { return }
        Task { @MainActor in
            // Wait for the async screen content and its focus geometry to settle.
            await Task.yield()
            guard !Task.isCancelled else { return }
            resetFocus(in: namespace)
        }
    }
    #endif
}

extension View {
    func tvOSDefaultPlayFocus(in namespace: Namespace.ID, id: String = "tvOS.primary-play", enabled: Bool = true) -> some View {
        modifier(TVOSDefaultPlayFocus(namespace: namespace, layoutID: id, enabled: enabled))
    }
}
