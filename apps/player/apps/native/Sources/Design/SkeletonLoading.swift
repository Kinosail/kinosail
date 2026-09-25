import SwiftUI

private struct SkeletonShimmer: ViewModifier {
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.scenePhase) private var scenePhase
    @State private var sweep = -1.0

    func body(content: Content) -> some View {
        content.overlay {
            if !reduceMotion && scenePhase == .active {
                GeometryReader { geometry in
                    LinearGradient(colors: [.clear, KinoTheme.text.opacity(0.12), .clear], startPoint: .leading, endPoint: .trailing)
                        .frame(width: geometry.size.width * 0.55)
                        .offset(x: sweep * geometry.size.width)
                }
                .mask(content)
                .allowsHitTesting(false)
                .accessibilityHidden(true)
                .task(id: scenePhase) {
                    sweep = -1
                    withAnimation(.linear(duration: 1.7).repeatForever(autoreverses: false)) { sweep = 1.5 }
                }
            }
        }
    }
}

private struct SkeletonLoadingStatus: ViewModifier {
    let title: String
    let shimmers: Bool
    func body(content: Content) -> some View {
        Group {
            if shimmers { content.modifier(SkeletonShimmer()) }
            else { content }
        }
        .accessibilityElement(children: .ignore).accessibilityLabel(title)
    }
}

extension View {
    func skeletonShimmer() -> some View { modifier(SkeletonShimmer()) }
    func skeletonLoading(_ title: String, shimmers: Bool = true) -> some View {
        modifier(SkeletonLoadingStatus(title: title, shimmers: shimmers))
    }
}

struct SkeletonRow: View {
    enum Kind { case media, list, form, text }
    var kind: Kind = .list
    var status: String? = nil

    var body: some View {
        HStack(spacing: 16) {
            if kind == .media {
                RoundedRectangle(cornerRadius: 10).fill(KinoTheme.surface).frame(width: 76, height: 114)
            }
            VStack(alignment: .leading, spacing: 8) {
                RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(maxWidth: kind == .form ? 150 : 200).frame(height: 20)
                if kind != .form { RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(maxWidth: 120).frame(height: 14) }
                if kind == .media { Capsule().fill(KinoTheme.surface).frame(width: 110, height: 32) }
            }
            Spacer(minLength: 0)
            if kind == .form { Capsule().fill(KinoTheme.surface).frame(width: 72, height: 24) }
            if kind == .list { Circle().fill(KinoTheme.surface).frame(width: 36, height: 36) }
        }
        .frame(minHeight: kind == .media ? 114 : 44)
        .skeletonShimmer()
        .accessibilityElement(children: .ignore)
        .accessibilityLabel(status ?? "")
        .accessibilityHidden(status == nil)
    }
}
