import SwiftUI

private struct TVOSDefaultPlayFocus: ViewModifier {
    let namespace: Namespace.ID
    let enabled: Bool

    func body(content: Content) -> some View {
        #if os(tvOS)
        content.prefersDefaultFocus(enabled, in: namespace)
        #else
        content
        #endif
    }
}

extension View {
    func tvOSDefaultPlayFocus(in namespace: Namespace.ID, enabled: Bool = true) -> some View {
        modifier(TVOSDefaultPlayFocus(namespace: namespace, enabled: enabled))
    }
}
