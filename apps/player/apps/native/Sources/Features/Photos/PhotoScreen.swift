import SwiftUI

struct PhotoScreen: View {
    let itemID: String
    @Environment(AppSession.self) private var session
    @State private var image: UIImage?
    @State private var title = "Photo"
    @State private var failure: String?
    @State private var revision = 0
    #if os(tvOS)
    @State private var zoomed = false
    @State private var offset = CGSize.zero
    #endif

    var body: some View {
        Group {
            if let image {
                #if os(iOS)
                ZoomablePhoto(image: image, title: title)
                #else
                GeometryReader { geometry in
                    Image(uiImage: image).resizable().scaledToFit()
                        .frame(width: geometry.size.width, height: geometry.size.height)
                        .scaleEffect(zoomed ? 2 : 1).offset(offset).clipped()
                        .accessibilityLabel(title)
                        .accessibilityHint(zoomed ? "Press Play/Pause to fit. Use the remote to pan." : "Press Play/Pause to zoom.")
                        .focusable().onPlayPauseCommand { zoomed.toggle(); offset = .zero }
                        .onMoveCommand(perform: zoomed ? { direction in
                            switch direction {
                            case .left: offset.width = min(geometry.size.width / 2, offset.width + 100)
                            case .right: offset.width = max(-geometry.size.width / 2, offset.width - 100)
                            case .up: offset.height = min(geometry.size.height / 2, offset.height + 100)
                            case .down: offset.height = max(-geometry.size.height / 2, offset.height - 100)
                            @unknown default: break
                            }
                        } : nil)
                }
                .toolbar { Button(zoomed ? "Fit photo" : "Zoom in") { zoomed.toggle(); offset = .zero } }
                #endif
            } else if let failure { RetryState(message: failure) { revision += 1 } }
            else { Text("Opening photo…").foregroundStyle(.secondary) }
        }
        .frame(maxWidth: .infinity, maxHeight: .infinity).background(.black)
        .preferredColorScheme(.dark)
        .navigationTitle(title)
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar(.hidden, for: .tabBar)
        #endif
        .task(id: "\(session.profileKey ?? ""):\(itemID):\(revision)") {
            image = nil; failure = nil
            guard let client = session.client else { return }
            do {
                var displayedCache = false
                if let saved = try? await client.item(id: itemID, policy: .cached), saved.kind == .photo, !saved.stream.isEmpty,
                   let decoded = try? await session.artwork.image(path: saved.stream, client: client, dimension: 4096) {
                    try Task.checkCancellation()
                    title = saved.title; image = UIImage(cgImage: decoded); displayedCache = true
                }
                do {
                    let item = try await client.item(id: itemID, policy: .automatic)
                    guard item.kind == .photo, !item.stream.isEmpty else { throw ClientError.invalidResponse }
                    let decoded = try await session.artwork.image(path: item.stream, client: client, dimension: 4096)
                    try Task.checkCancellation()
                    title = item.title; image = UIImage(cgImage: decoded)
                } catch {
                    if error is CancellationError { throw error }
                    if !displayedCache { throw error }
                }
            } catch is CancellationError {} catch { failure = AppSession.message(error) }
        }
    }
}

#if os(iOS)
private struct ZoomablePhoto: UIViewRepresentable {
    let image: UIImage
    let title: String
    func makeUIView(context: Context) -> PhotoScrollView { PhotoScrollView() }
    func updateUIView(_ view: PhotoScrollView, context: Context) {
        if view.photo.image !== image { view.photo.image = image; view.setZoomScale(1, animated: false); view.setNeedsLayout() }
        view.photo.accessibilityLabel = title
    }
}

private final class PhotoScrollView: UIScrollView, UIScrollViewDelegate {
    let photo = UIImageView()
    private var previousSize = CGSize.zero
    init() {
        super.init(frame: .zero)
        delegate = self; minimumZoomScale = 1; maximumZoomScale = 4
        backgroundColor = .black; photo.contentMode = .scaleAspectFit; photo.isAccessibilityElement = true
        addSubview(photo)
        let doubleTap = UITapGestureRecognizer(target: self, action: #selector(toggleZoom))
        doubleTap.numberOfTapsRequired = 2; addGestureRecognizer(doubleTap)
        photo.accessibilityCustomActions = [UIAccessibilityCustomAction(name: "Toggle zoom", target: self, selector: #selector(accessibleZoom))]
    }
    required init?(coder: NSCoder) { fatalError("init(coder:) is unavailable") }
    override func layoutSubviews() {
        super.layoutSubviews()
        if previousSize != bounds.size {
            previousSize = bounds.size; setZoomScale(1, animated: false)
            photo.frame = CGRect(origin: .zero, size: bounds.size); contentSize = bounds.size
        }
    }
    func viewForZooming(in scrollView: UIScrollView) -> UIView? { photo }
    @objc private func toggleZoom() { setZoomScale(zoomScale > 1 ? 1 : 2, animated: !UIAccessibility.isReduceMotionEnabled) }
    @objc private func accessibleZoom() -> Bool { toggleZoom(); return true }
}
#endif
