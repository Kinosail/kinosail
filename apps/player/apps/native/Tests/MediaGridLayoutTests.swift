import SwiftUI
import Testing
@testable import KinosailPlayer

#if os(iOS)
struct MediaGridLayoutTests {
    @Test @MainActor func phonePosterGridFitsThreeCardsWithoutCrowdingNarrowWidths() {
        #expect(renderedHeight(width: 335, landscape: false, accessibility: false) == 10)
        #expect(renderedHeight(width: 280, landscape: false, accessibility: false) == 48)
        #expect(renderedHeight(width: 335, landscape: false, accessibility: true) == 86)
        #expect(renderedHeight(width: 335, landscape: true, accessibility: false) == 86)
    }

    @MainActor private func renderedHeight(width: CGFloat, landscape: Bool, accessibility: Bool) -> CGFloat {
        let grid = LazyVGrid(columns: MediaGrid.columns(landscape: landscape, accessibility: accessibility), spacing: 28) {
            ForEach(0..<3) { _ in Rectangle().frame(height: 10) }
        }.frame(width: width)
        return ImageRenderer(content: grid).uiImage?.size.height ?? -1
    }
}
#endif
