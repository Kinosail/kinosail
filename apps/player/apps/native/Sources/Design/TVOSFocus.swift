import SwiftUI

private struct TVOSDefaultPlayFocus: ViewModifier {
    let namespace: Namespace.ID
    let layoutID: String
    let enabled: Bool
    func body(content: Content) -> some View {
        #if os(tvOS)
        content
            .id(layoutID)
            .accessibilityIdentifier(layoutID)
            .prefersDefaultFocus(enabled, in: namespace)
        #else
        content
        #endif
    }
}

extension View {
    func tvOSDefaultPlayFocus(in namespace: Namespace.ID, id: String = "tvOS.primary-play", enabled: Bool = true) -> some View {
        modifier(TVOSDefaultPlayFocus(namespace: namespace, layoutID: id, enabled: enabled))
    }
}
