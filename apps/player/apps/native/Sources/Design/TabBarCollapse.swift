#if os(iOS)
import SwiftUI

@Observable final class TabBarCollapse {
    private(set) var collapsible = true
    @ObservationIgnored private var anchor: CGFloat = 0

    func track(_ offset: CGFloat) {
        let travel = offset - anchor
        if offset <= 24 || travel > 24 {
            anchor = offset
            if !collapsible { collapsible = true }
        } else if travel < -24 {
            anchor = offset
            expand()
        } else if collapsible ? offset > anchor : offset < anchor {
            anchor = offset
        }
    }

    func expand() {
        if collapsible { collapsible = false }
    }
}

extension EnvironmentValues {
    @Entry var tabBarCollapse: TabBarCollapse?
}

private struct TabBarCollapseTracking: ViewModifier {
    @Environment(\.tabBarCollapse) private var state

    func body(content: Content) -> some View {
        content.onScrollGeometryChange(for: CGFloat.self) { geometry in
            (geometry.contentOffset.y + geometry.contentInsets.top).rounded()
        } action: { _, offset in state?.track(offset) }
    }
}

extension View {
    func tabBarCollapses() -> some View { modifier(TabBarCollapseTracking()) }
}
#else
import SwiftUI

extension View {
    func tabBarCollapses() -> some View { self }
}
#endif
