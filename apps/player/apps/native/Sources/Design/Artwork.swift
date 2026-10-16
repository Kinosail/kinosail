import SwiftUI

struct Artwork: View {
    let path: String
    var symbol = "film"
    var ratio: CGFloat = 2 / 3
    var dimension = 1600
    var fillsFrame = false
    var isBackdrop = false
    var canvasSize: CGSize? = nil
    @Environment(AppSession.self) private var session
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @State private var image: CGImage?
    @State private var imageIdentity = UUID()
    @State private var loadedProfileKey: String?
    @State private var loading = true
    @State private var generation = UUID()

    private var instant: CGImage? {
        guard image == nil, !path.isEmpty, let client = session.client else { return nil }
        return session.artwork.cachedImage(path: path, client: client, dimension: dimension)
    }

    var body: some View {
        let shown = image ?? instant
        canvas
            .overlay {
                if let shown {
                    Image(decorative: shown, scale: 1).resizable()
                        .aspectRatio(contentMode: Self.contentMode(fillsFrame: fillsFrame))
                        .id(imageIdentity).transition(.opacity)
                }
                else if !isBackdrop { Rectangle().fill(KinoTheme.surface).overlay { if !loading { Image(systemName: symbol).font(.largeTitle).foregroundStyle(KinoTheme.muted) } } }
            }
            .clipped()
            .accessibilityHidden(true)
            .task(id: "\(session.profileKey ?? ""):\(path):\(dimension)") {
                let attempt = UUID()
                generation = attempt
                if !isBackdrop || loadedProfileKey != session.profileKey || path.isEmpty { image = nil }
                loadedProfileKey = session.profileKey
                guard !path.isEmpty, let client = session.client else { loading = false; return }
                if image == nil, let cached = session.artwork.cachedImage(path: path, client: client, dimension: dimension) { image = cached }
                loading = image == nil
                defer { if generation == attempt { loading = false } }
                do {
                    let decoded = try await session.artwork.image(path: path, client: client, dimension: dimension)
                    try Task.checkCancellation()
                    guard generation == attempt, decoded !== image else { return }
                    withAnimation(isBackdrop && !reduceMotion ? .easeInOut(duration: 0.45) : nil) {
                        image = decoded
                        imageIdentity = UUID()
                    }
                } catch {
                    if generation == attempt && !Task.isCancelled { image = nil }
                }
            }
    }
    @ViewBuilder private var canvas: some View {
        if let canvasSize {
            Color.clear.frame(width: canvasSize.width, height: canvasSize.height)
        } else {
            Color.clear.aspectRatio(ratio, contentMode: .fit)
        }
    }

    static func contentMode(fillsFrame: Bool) -> ContentMode {
        fillsFrame ? .fill : .fit
    }
}
