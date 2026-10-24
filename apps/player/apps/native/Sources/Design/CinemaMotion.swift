import SwiftUI

private struct ZoomNamespaceKey: EnvironmentKey {
    static let defaultValue: Namespace.ID? = nil
}

extension EnvironmentValues {
    var zoomNamespace: Namespace.ID? {
        get { self[ZoomNamespaceKey.self] }
        set { self[ZoomNamespaceKey.self] = newValue }
    }
}

extension ScreenDestination {
    var zooms: Bool {
        switch self {
        case .detail, .show, .album, .collection: true
        default: false
        }
    }
}

private struct ZoomHost: ViewModifier {
    @Namespace private var namespace

    func body(content: Content) -> some View {
        content.environment(\.zoomNamespace, namespace)
    }
}

private struct ZoomSource: ViewModifier {
    let destination: ScreenDestination
    @Environment(\.zoomNamespace) private var namespace

    func body(content: Content) -> some View {
        #if os(iOS)
        if let namespace, destination.zooms {
            content.matchedTransitionSource(id: destination, in: namespace) { source in
                source.clipShape(.rect(cornerRadius: 12))
            }
        } else { content }
        #else
        content
        #endif
    }
}

private struct ZoomDestination: ViewModifier {
    let destination: ScreenDestination
    @Environment(\.zoomNamespace) private var namespace
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    func body(content: Content) -> some View {
        #if os(iOS)
        if let namespace, destination.zooms, !reduceMotion {
            content.navigationTransition(.zoom(sourceID: destination, in: namespace))
        } else { content }
        #else
        content
        #endif
    }
}

struct PressableCardStyle: ButtonStyle {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed && !reduceMotion ? 0.965 : 1)
            .brightness(configuration.isPressed ? -0.06 : 0)
            .animation(reduceMotion ? nil : .spring(response: 0.28, dampingFraction: 0.72), value: configuration.isPressed)
    }
}

struct UnwatchedMark: View {
    #if os(tvOS)
    private let size: CGFloat = 18
    #else
    private let size: CGFloat = 11
    #endif

    var body: some View {
        Circle()
            .fill(KinoTheme.signal)
            .frame(width: size, height: size)
            .overlay { Circle().strokeBorder(.black.opacity(0.45), lineWidth: size / 6) }
            .shadow(color: KinoTheme.signal.opacity(0.55), radius: size / 2)
            .padding(size * 0.8)
    }
}

extension View {
    func zoomTransitionHost() -> some View { modifier(ZoomHost()) }
    func zoomSource(_ destination: ScreenDestination) -> some View { modifier(ZoomSource(destination: destination)) }
    func zoomDestination(_ destination: ScreenDestination) -> some View { modifier(ZoomDestination(destination: destination)) }
}
