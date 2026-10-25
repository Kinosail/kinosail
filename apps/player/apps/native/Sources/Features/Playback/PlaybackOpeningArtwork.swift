#if os(iOS)
import SwiftUI

struct PlaybackOpeningArtwork: View {
    let visible: Bool
    @Environment(AppSession.self) private var session
    @Environment(\.accessibilityReduceMotion) private var reduceMotion

    private var item: MediaItem? { session.player.currentItem ?? session.video.preview(for: session.presentedVideoID) }
    private var path: String {
        guard let item, item.kind == .video, item.landscapeArtwork != item.poster else { return "" }
        return item.landscapeArtwork
    }

    var body: some View {
        GeometryReader { geometry in
            let width = min(geometry.size.width, geometry.size.height * 16 / 9)
            let size = CGSize(width: width, height: width * 9 / 16)
            if !path.isEmpty {
                Artwork(path: path, ratio: 16 / 9, dimension: 1600, fillsFrame: true, isBackdrop: true, canvasSize: size)
                    .frame(width: size.width, height: size.height)
                    .overlay { Color.black.opacity(0.45) }
                    .position(x: geometry.size.width / 2, y: geometry.size.height / 2)
                    .opacity(visible ? 1 : 0)
                    .animation(reduceMotion ? nil : .easeOut(duration: 0.35), value: visible)
            }
        }
        .ignoresSafeArea()
        .allowsHitTesting(false)
        .accessibilityHidden(true)
    }
}
#endif
