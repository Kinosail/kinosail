#if os(iOS)
import SwiftUI
import Testing
@testable import KinosailPlayer

// Before gesture edits: headless native journeys cannot inject actual SwiftUI
// drags. This isolated check protects accidental close and control-region conflicts.
@MainActor struct PlaybackDismissalTests {
    @Test func shortHorizontalUpwardAndControlRegionDragsCannotDismiss() {
        let dismissal = PlaybackDismissal()
        var closes = 0
        let size = CGSize(width: 402, height: 874)
        let cases: [(CGSize, CGPoint)] = [
            (CGSize(width: 0, height: 99), CGPoint(x: 120, y: 200)),
            (CGSize(width: 200, height: 150), CGPoint(x: 120, y: 200)),
            (CGSize(width: 0, height: -200), CGPoint(x: 120, y: 200)),
            (CGSize(width: 0, height: 200), CGPoint(x: 120, y: 40)),
            (CGSize(width: 0, height: 200), CGPoint(x: 120, y: 750)),
        ]
        for (translation, start) in cases {
            dismissal.dragEnded(translation: translation, start: start, size: size, excludedBottom: 144, close: { closes += 1 })
        }
        #expect(closes == 0)
        #expect(!dismissal.closing)
        // Cancellation does not call onEnded and therefore never calls close.
        #expect(!dismissal.closing)
    }

    @Test func downwardBackgroundGestureAndCloseButtonDismissOnlyOnce() {
        let dismissal = PlaybackDismissal()
        var closes = 0
        let close = { closes += 1 }
        dismissal.dragEnded(translation: CGSize(width: 20, height: 140), start: CGPoint(x: 80, y: 100),
                            size: CGSize(width: 874, height: 402), excludedBottom: 144, close: close)
        dismissal.requestClose(close)
        dismissal.dragEnded(translation: CGSize(width: 0, height: 200), start: CGPoint(x: 80, y: 100),
                            size: CGSize(width: 874, height: 402), excludedBottom: 144, close: close)
        #expect(closes == 1)
        #expect(dismissal.closing)
    }
}
#endif
