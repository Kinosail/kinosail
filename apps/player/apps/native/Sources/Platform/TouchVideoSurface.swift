#if os(iOS)
import AVKit
import SwiftUI

/// AVFoundation owns rendering, aspect ratio and HDR; captions stay inside the visible picture.
final class TouchVideoSurface: UIView {
    override class var layerClass: AnyClass { AVPlayerLayer.self }
    var playerLayer: AVPlayerLayer { layer as! AVPlayerLayer }
    let captions = UILabel()
    var controlsVisible = true
    var controlsInset: CGFloat = 144
    var visibilityChanged: ((Bool) -> Void)?

    override init(frame: CGRect) {
        super.init(frame: frame)
        backgroundColor = .black
        captions.textColor = .white
        captions.backgroundColor = UIColor.black.withAlphaComponent(0.72)
        captions.textAlignment = .center
        captions.numberOfLines = 0
        captions.font = .preferredFont(forTextStyle: .title3)
        captions.adjustsFontForContentSizeCategory = true
        captions.isAccessibilityElement = true
        captions.isUserInteractionEnabled = false
        addSubview(captions)
    }
    required init?(coder: NSCoder) { nil }

    override func didMoveToWindow() {
        super.didMoveToWindow()
        visibilityChanged?(window != nil)
    }

    override func layoutSubviews() {
        super.layoutSubviews()
        let picture = playerLayer.videoRect.isEmpty ? bounds : playerLayer.videoRect
        let division: CGRect?
        if #available(iOS 27.1, *) { division = reservedRegions(kind: .division).first?.frame }
        else { division = nil }
        captions.frame = Self.captionFrame(in: bounds, safeAreaInsets: safeAreaInsets, picture: picture,
                                          division: division, controlsVisible: controlsVisible, controlsInset: controlsInset) { size in
            captions.sizeThatFits(size)
        }
    }

    static func captionFrame(in bounds: CGRect, safeAreaInsets: UIEdgeInsets, picture: CGRect,
                             division: CGRect? = nil, controlsVisible: Bool, controlsInset: CGFloat,
                             fitting: (CGSize) -> CGSize) -> CGRect {
        let safe = bounds.inset(by: safeAreaInsets)
        var visible = picture.intersection(safe)
        guard !visible.isNull, !visible.isEmpty else { return .zero }
        if let division, division.height > division.width, division.intersects(visible) {
            let left = CGRect(x: visible.minX, y: visible.minY,
                              width: max(0, min(visible.maxX, division.minX) - visible.minX), height: visible.height)
            let right = CGRect(x: max(visible.minX, division.maxX), y: visible.minY,
                               width: max(0, visible.maxX - max(visible.minX, division.maxX)), height: visible.height)
            visible = left.width >= right.width ? left : right
        }
        guard !visible.isEmpty else { return .zero }
        let width = max(0, min(visible.width - 32, picture.width * 0.85))
        let size = fitting(CGSize(width: width, height: bounds.height / 2))
        let bottom = min(visible.maxY - 20, safe.maxY - (controlsVisible ? controlsInset : 24))
        return CGRect(x: visible.midX - width / 2, y: max(visible.minY, bottom - size.height), width: width, height: size.height)
    }
}

struct TouchVideoView: UIViewRepresentable {
    let player: AVPlayer?
    let presentation: PlayerPresentation
    let controlsVisible: Bool
    let controlsInset: CGFloat
    let restore: () -> Void

    func makeUIView(context: Context) -> TouchVideoSurface {
        presentation.restore = restore
        if let player { presentation.attach(player) }
        return presentation.videoView
    }
    func updateUIView(_ view: TouchVideoSurface, context: Context) {
        presentation.restore = restore
        if let player { presentation.attach(player) }
        view.controlsVisible = controlsVisible
        view.controlsInset = controlsInset
        view.captions.text = presentation.captionText
        view.captions.isHidden = presentation.captionText.isEmpty || presentation.pictureInPicture
        view.setNeedsLayout()
    }
    func makeCoordinator() -> PlayerPresentation { presentation }
    static func dismantleUIView(_ view: TouchVideoSurface, coordinator: PlayerPresentation) {
        coordinator.visible = false
        if !coordinator.pictureInPicture { coordinator.attach(nil) }
    }
}
#endif
