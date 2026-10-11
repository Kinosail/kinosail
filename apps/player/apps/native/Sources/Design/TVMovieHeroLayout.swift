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

extension View {
    func tvMovieHeroBleed() -> some View {
        // Expand only the decorative hero; the surrounding shelves keep the
        // screen inset. The shared layout retains the previous 104pt text inset.
        padding(.horizontal, -KinoTheme.contentPadding)
            .padding(.top, -KinoTheme.contentPadding)
    }
}
#endif
