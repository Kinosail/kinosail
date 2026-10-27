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
    @Environment(\.scenePhase) private var scenePhase
    @State private var retry = 0
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
            .onChange(of: scenePhase) { _, phase in if phase == .active && image == nil && !loading && !path.isEmpty { retry += 1 } }
            .onChange(of: session.connection.unavailable) { _, unavailable in if !unavailable && image == nil && !loading && !path.isEmpty { retry += 1 } }
            .task(id: "\(session.profileKey ?? ""):\(session.client == nil):\(path):\(dimension):\(retry)") {
                let attempt = UUID()
                generation = attempt
                if !isBackdrop || loadedProfileKey != session.profileKey || path.isEmpty { image = nil }
                loadedProfileKey = session.profileKey
                guard !path.isEmpty, let client = session.client else { loading = false; return }
                if image == nil, let cached = session.artwork.cachedImage(path: path, client: client, dimension: dimension) { image = cached }
                loading = image == nil
                defer { if generation == attempt { loading = false } }
                for failures in 0... {
                    let decoded = try? await session.artwork.image(path: path, client: client, dimension: dimension)
                    guard generation == attempt, !Task.isCancelled else { return }
                    if let decoded {
                        guard decoded !== image else { return }
                        withAnimation(isBackdrop && !reduceMotion ? .easeInOut(duration: 0.45) : nil) {
                            image = decoded
                            imageIdentity = UUID()
                        }
                        return
                    }
                    if image != nil { return }
                    if failures >= Self.retryDelays.count { loading = false }
                    do { try await Task.sleep(for: Self.retryDelay(after: failures)) } catch { return }
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

    static let retryDelays: [Duration] = [.seconds(1), .seconds(3), .seconds(8), .seconds(20)]

    static func retryDelay(after failures: Int) -> Duration {
        failures < retryDelays.count ? retryDelays[failures] : .seconds(30)
    }

    static func contentMode(fillsFrame: Bool) -> ContentMode {
        fillsFrame ? .fill : .fit
    }
}
