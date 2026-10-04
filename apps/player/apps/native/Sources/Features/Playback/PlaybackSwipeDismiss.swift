#if os(iOS)
import Observation
import SwiftUI

@MainActor @Observable
final class PlaybackDismissal {
    private(set) var closing = false

    func requestClose(_ close: () -> Void) {
        guard !closing else { return }
        closing = true
        close()
    }

    func dragEnded(translation: CGSize, start: CGPoint, size: CGSize, excludedBottom: CGFloat, close: () -> Void) {
        guard translation.height >= 100, translation.height > abs(translation.width) * 1.5,
              start.y >= 60, start.y < size.height - excludedBottom else { return }
        requestClose(close)
    }
}

/// Only the video-background hit surface receives this gesture; controls stay above it.
struct PlaybackSwipeDismiss: ViewModifier {
    let dismissal: PlaybackDismissal
    let size: CGSize
    let excludedBottom: CGFloat
    let enabled: Bool
    let close: () -> Void
    @Environment(\.accessibilityVoiceOverEnabled) private var voiceOver

    func body(content: Content) -> some View {
        content.simultaneousGesture(DragGesture(minimumDistance: 30).onEnded { value in
            guard enabled, !voiceOver else { return }
            dismissal.dragEnded(translation: value.translation, start: value.startLocation,
                                size: size, excludedBottom: excludedBottom, close: close)
        })
    }
}
#endif
