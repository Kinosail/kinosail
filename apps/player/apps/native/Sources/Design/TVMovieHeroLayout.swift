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

// Safe-area expansion belongs to the decorative plane. Its measured canvas
// and clipping expand together; foreground layout and focus retain their inset.
struct TVMovieHeroBackdrop<Backdrop: View>: View {
    @ViewBuilder let backdrop: (CGSize) -> Backdrop

    var body: some View {
        GeometryReader { geometry in
            backdrop(geometry.size)
                .frame(width: geometry.size.width, height: geometry.size.height)
                .clipped()
        }
        .ignoresSafeArea(.container, edges: [.top, .horizontal])
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}

extension View {
    func tvMovieHeroBleed() -> some View {
        // Expand only the decorative hero; the surrounding shelves keep the
        // screen inset. The shared layout retains the previous 104pt text inset.
        padding(.horizontal, -KinoTheme.contentPadding)
            .padding(.top, -KinoTheme.contentPadding)
    }
}
#endif
