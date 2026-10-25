#if os(tvOS)
import SwiftUI

struct TVFlatButtonStyle: ButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        configuration.label
            .scaleEffect(configuration.isPressed ? 0.96 : 1)
            .animation(.snappy(duration: 0.16), value: configuration.isPressed)
    }
}

struct TVCapsuleButtonStyle: ButtonStyle {
    var prominent = false

    func makeBody(configuration: Configuration) -> some View {
        TVCapsuleButton(configuration: configuration, prominent: prominent)
    }
}

private struct TVCapsuleButton: View {
    let configuration: ButtonStyleConfiguration
    let prominent: Bool
    @Environment(\.isFocused) private var focused
    @Environment(\.isEnabled) private var enabled
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private var fill: Color {
        if focused { return .white }
        return prominent ? KinoTheme.tvOSPrimaryFill : .white.opacity(0.14)
    }

    var body: some View {
        configuration.label
            .font(.callout.weight(.semibold))
            .labelStyle(TVCapsuleLabelStyle())
            .padding(.horizontal, 26).padding(.vertical, 12)
            .frame(minHeight: 80)
            .foregroundStyle(focused || prominent ? KinoTheme.tvOSPrimaryInk : KinoTheme.text)
            .background(fill, in: Capsule())
            .opacity(enabled ? 1 : 0.4)
            .scaleEffect(configuration.isPressed ? 0.97 : focused && !reduceMotion ? 1.06 : 1)
            .animation(.snappy(duration: 0.2), value: focused)
            .animation(.snappy(duration: 0.14), value: configuration.isPressed)
    }
}

private struct TVCapsuleLabelStyle: LabelStyle {
    func makeBody(configuration: Configuration) -> some View {
        HStack(spacing: 12) {
            configuration.icon
            configuration.title
        }
    }
}

extension View {
    func tvCapsule(prominent: Bool = false) -> some View {
        buttonStyle(TVCapsuleButtonStyle(prominent: prominent)).focusEffectDisabled()
    }

    func tvFlat() -> some View {
        buttonStyle(TVFlatButtonStyle()).focusEffectDisabled()
    }
}
#endif
