#if os(iOS)
import SwiftUI

@Observable final class SpotlightScroll {
    var offset: CGFloat = 0
}

extension View {
    func spotlightScroll(_ scroll: SpotlightScroll) -> some View {
        onScrollGeometryChange(for: CGFloat.self) { geometry in
            let value = geometry.contentOffset.y + geometry.contentInsets.top
            return min(max(value, -400), 600).rounded()
        } action: { _, value in scroll.offset = value }
    }
}

struct CinemaSpotlight<Actions: View, Secondary: View>: View {
    let item: MediaItem
    let artwork: String
    var eyebrow: String?
    var title: String?
    var subtitle: String?
    var scroll: SpotlightScroll?
    @ViewBuilder let actions: () -> Actions
    @ViewBuilder var secondary: () -> Secondary
    @Environment(\.horizontalSizeClass) private var sizeClass
    @Environment(\.verticalSizeClass) private var verticalSize
    @Environment(\.dynamicTypeSize) private var dynamicType
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.colorScheme) private var colorScheme
    @ScaledMetric(relativeTo: .largeTitle) private var titleSize = 40.0
    @State private var width: CGFloat = 0

    private var geometry: SpotlightGeometry {
        SpotlightGeometry(width: width, wide: sizeClass == .regular && !dynamicType.isAccessibilitySize,
                          short: verticalSize == .compact)
    }
    private var wide: Bool { geometry.wide }
    private var artHeight: CGFloat { geometry.artHeight }
    private static var bleed: CGFloat { SpotlightGeometry.bleed }

    var body: some View {
        geometry.layout(width: $width) { art } information: { information }
    }

    private var art: some View {
        Color.clear
            .frame(height: artHeight)
            .overlay(alignment: .bottom) {
                Artwork(path: artwork, ratio: geometry.canvasWidth / max(artHeight + Self.bleed, 1),
                        dimension: 1600, fillsFrame: true, isBackdrop: true,
                        canvasSize: CGSize(width: geometry.canvasWidth, height: artHeight + Self.bleed))
                    .overlay(alignment: .top) {
                        LinearGradient(colors: [KinoTheme.background.opacity(colorScheme == .dark ? 0.7 : 0.55), .clear],
                                       startPoint: .top, endPoint: .bottom)
                            .frame(height: Self.bleed + 64)
                    }
                    .mask { geometry.fade }
                    .modifier(SpotlightMotion(scroll: reduceMotion ? nil : scroll, height: artHeight + Self.bleed))
            }
            .allowsHitTesting(false)
            .accessibilityHidden(true)
    }

    private var information: some View {
        VStack(alignment: .leading, spacing: 14) {
            VStack(alignment: .leading, spacing: 8) {
                if let eyebrow {
                    Text(eyebrow.uppercased())
                        .font(.caption.weight(.heavy)).tracking(1.6)
                        .foregroundStyle(KinoTheme.signal)
                }
                Text(title ?? item.title)
                    .font(.system(size: titleSize, weight: .bold, design: .rounded))
                    .tracking(-0.6)
                    .lineLimit(dynamicType.isAccessibilitySize ? nil : 3)
                    .minimumScaleFactor(dynamicType.isAccessibilitySize ? 1 : 0.8)
                    .fixedSize(horizontal: false, vertical: true)
                    .shadow(color: .black.opacity(colorScheme == .dark ? 0.45 : 0), radius: 14, y: 2)
                    .accessibilityAddTraits(.isHeader)
                let summary = subtitle ?? item.subtitle
                if !summary.isEmpty {
                    Text(summary).font(.subheadline.weight(.medium)).foregroundStyle(KinoTheme.muted)
                }
            }
            if item.progress.seconds > 0 && !item.progress.watched {
                WatchPosition(item: item).frame(maxWidth: 360, alignment: .leading)
            }
            ViewThatFits(in: .horizontal) {
                HStack(spacing: 12) { actions(); secondary() }
                VStack(alignment: .leading, spacing: 12) {
                    HStack(spacing: 12) { actions() }
                    HStack(spacing: 12) { secondary() }
                }
                VStack(alignment: .leading, spacing: 12) { actions(); secondary() }
            }
            .fixedSize(horizontal: false, vertical: true)
            .controlSize(.large)
        }
        .frame(maxWidth: .infinity, alignment: .leading)
    }
}

extension MediaItem {
    var spotlightArtwork: String {
        guard kind == .video || kind == .show else { return "" }
        if !show.isEmpty && !artwork.isEmpty { return landscapeArtwork }
        return backdrop.isEmpty ? poster : backdrop
    }
}

struct SpotlightGeometry {
    static var bleed: CGFloat { 220 }
    let width: CGFloat
    let wide: Bool
    var short = false
    var canvasWidth: CGFloat { short ? width + 240 : width }
    var artHeight: CGFloat {
        guard width > 0 else { return 0 }
        if short { return min(width * 0.34, 280) }
        return wide ? min(max(width * 0.46, 380), 640) : width * 0.86
    }
    var overlap: CGFloat { wide ? artHeight * 0.52 : min(artHeight * 0.34, 132) }
    var fade: LinearGradient {
        LinearGradient(stops: [.init(color: .black, location: 0), .init(color: .black, location: wide ? 0.5 : 0.62),
                               .init(color: .black.opacity(0.55), location: wide ? 0.78 : 0.82), .init(color: .clear, location: 1)],
                       startPoint: .top, endPoint: .bottom)
    }

    func layout(width binding: Binding<CGFloat>, @ViewBuilder art: () -> some View,
                @ViewBuilder information: () -> some View) -> some View {
        VStack(alignment: .leading, spacing: -overlap) {
            art()
            information()
                .frame(maxWidth: wide ? 600 : .infinity, alignment: .leading)
                .padding(.horizontal, KinoTheme.contentPadding)
        }
        .onGeometryChange(for: CGFloat.self) { $0.size.width } action: { binding.wrappedValue = $0 }
        .padding(.horizontal, -KinoTheme.contentPadding)
        .foregroundStyle(KinoTheme.text)
    }
}

struct SpotlightSkeleton: View {
    var eyebrow = true
    @Environment(\.horizontalSizeClass) private var sizeClass
    @Environment(\.verticalSizeClass) private var verticalSize
    @Environment(\.dynamicTypeSize) private var dynamicType
    @State private var width: CGFloat = 0

    var body: some View {
        let geometry = SpotlightGeometry(width: width, wide: sizeClass == .regular && !dynamicType.isAccessibilitySize,
                                         short: verticalSize == .compact)
        geometry.layout(width: $width) {
            Color.clear.frame(height: geometry.artHeight)
                .overlay(alignment: .bottom) {
                    KinoTheme.surface.frame(width: geometry.canvasWidth, height: geometry.artHeight + SpotlightGeometry.bleed)
                        .mask { geometry.fade }
                }
        } information: {
            VStack(alignment: .leading, spacing: 14) {
                VStack(alignment: .leading, spacing: 10) {
                    if eyebrow { line(80, 12) }
                    line(280, 42)
                    line(170, 18)
                }
                HStack(spacing: 12) {
                    Capsule().fill(KinoTheme.raised).frame(width: 132, height: 50)
                    Capsule().fill(KinoTheme.raised).frame(width: 108, height: 50)
                }
            }
            .frame(maxWidth: .infinity, alignment: .leading)
        }
        .accessibilityHidden(true)
    }

    private func line(_ width: CGFloat, _ height: CGFloat) -> some View {
        RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(maxWidth: width).frame(height: height)
    }
}

extension CinemaSpotlight where Secondary == EmptyView {
    init(item: MediaItem, artwork: String, eyebrow: String? = nil, title: String? = nil, subtitle: String? = nil,
         scroll: SpotlightScroll? = nil, @ViewBuilder actions: @escaping () -> Actions) {
        self.init(item: item, artwork: artwork, eyebrow: eyebrow, title: title, subtitle: subtitle,
                  scroll: scroll, actions: actions, secondary: { EmptyView() })
    }
}

private struct SpotlightMotion: ViewModifier {
    let scroll: SpotlightScroll?
    let height: CGFloat

    func body(content: Content) -> some View {
        if let scroll {
            let offset = scroll.offset
            content
                .scaleEffect(offset < 0 ? 1 + -offset / max(height, 1) : 1, anchor: .bottom)
                .offset(y: offset > 0 ? offset * 0.32 : 0)
        } else { content }
    }
}

struct SpotlightSecondaryButtonStyle: PrimitiveButtonStyle {
    func makeBody(configuration: Configuration) -> some View {
        Button(role: configuration.role, action: configuration.trigger) { configuration.label }
            .buttonStyle(.glass)
            .buttonBorderShape(.capsule)
    }
}
#endif
