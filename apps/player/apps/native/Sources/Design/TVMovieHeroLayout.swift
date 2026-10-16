#if os(tvOS)
import SwiftUI

struct TVMovieHeroLayout<Content: View>: View {
    @ViewBuilder let content: Content

    var body: some View {
        content
            .frame(maxWidth: 740, alignment: .leading)
            .padding(.vertical, 40)
            .padding(.horizontal, KinoTheme.contentPadding + 40)
            .frame(maxWidth: .infinity, minHeight: 600, alignment: .bottomLeading)
    }
}

private struct TVMovieHeroAnchor {
    let bounds: Anchor<CGRect>
    let path: String?
}

private struct TVMovieHeroAnchorKey: PreferenceKey {
    static var defaultValue: TVMovieHeroAnchor? { nil }
    static func reduce(value: inout TVMovieHeroAnchor?, nextValue: () -> TVMovieHeroAnchor?) {
        value = nextValue() ?? value
    }
}

private struct TVMovieHeroBackdrop: View {
    let anchor: TVMovieHeroAnchor?

    var body: some View {
        GeometryReader { geometry in
            ZStack(alignment: .topLeading) {
                if let anchor {
                    let frame = geometry[anchor.bounds]
                    let top = min(frame.minY, 0)
                    let canvas = CGSize(width: geometry.size.width,
                                        height: max(frame.maxY - top, geometry.size.width * 9 / 16))
                    TVBackdrop(path: anchor.path, canvas: canvas)
                        .frame(width: canvas.width, height: canvas.height)
                        .clipped()
                        .position(x: canvas.width / 2, y: top + canvas.height / 2)
                }
            }
            .frame(width: geometry.size.width, height: geometry.size.height, alignment: .topLeading)
            .clipped()
        }
        .ignoresSafeArea(.container, edges: [.top, .horizontal])
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

struct TVBackdrop: View {
    let path: String?
    let canvas: CGSize
    @Environment(\.colorScheme) private var colorScheme
    @Environment(\.colorSchemeContrast) private var contrast
    @Environment(\.accessibilityReduceTransparency) private var reduceTransparency

    var body: some View {
        ZStack {
            KinoTheme.surface
            if let path, !path.isEmpty, colorScheme == .dark && contrast != .increased && !reduceTransparency {
                ZStack(alignment: .leading) {
                    Artwork(path: path, ratio: 16 / 9, dimension: 4096,
                            fillsFrame: true, isBackdrop: true, canvasSize: canvas)
                    LinearGradient(stops: [
                        .init(color: .black.opacity(0.92), location: 0),
                        .init(color: .black.opacity(0.84), location: 0.75),
                        .init(color: .clear, location: 1)
                    ], startPoint: .leading, endPoint: .trailing)
                    .frame(width: min(canvas.width, 1120))
                    LinearGradient(stops: [
                        .init(color: .clear, location: 0),
                        .init(color: KinoTheme.background.opacity(0.12), location: 0.55),
                        .init(color: KinoTheme.background, location: 1)
                    ], startPoint: .top, endPoint: .bottom)
                }
            }
        }
        .mask {
            LinearGradient(stops: [
                .init(color: .black, location: 0),
                .init(color: .black, location: 0.72),
                .init(color: .clear, location: 1)
            ], startPoint: .top, endPoint: .bottom)
        }
        .accessibilityHidden(true)
    }
}

extension View {
    func tvMovieHeroArtwork(_ path: String? = nil) -> some View {
        anchorPreference(key: TVMovieHeroAnchorKey.self, value: .bounds) {
            TVMovieHeroAnchor(bounds: $0, path: path)
        }
    }

    func tvMovieHeroViewport() -> some View {
        clipped()
            .backgroundPreferenceValue(TVMovieHeroAnchorKey.self) { anchor in
                TVMovieHeroBackdrop(anchor: anchor)
            }
    }

    func tvMovieHeroBleed() -> some View {
        padding(.horizontal, -KinoTheme.contentPadding)
            .padding(.top, -KinoTheme.contentPadding)
    }
}
#endif
