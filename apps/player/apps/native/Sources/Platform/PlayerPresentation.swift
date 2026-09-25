import AVKit
import Observation

/// Retains the video surface and its system Picture in Picture session across navigation.
@MainActor @Observable
final class PlayerPresentation: NSObject, AVPlayerViewControllerDelegate {
    private(set) var pictureInPicture = false
    private(set) var captionText = ""
    private(set) var readyForDisplay = false
    @ObservationIgnored private var readinessObservation: NSKeyValueObservation?
    #if os(iOS)
    private(set) var pictureInPicturePossible = false
    private(set) var presentationMessage: String?
    @ObservationIgnored let videoView = TouchVideoSurface()
    @ObservationIgnored private var pip: AVPictureInPictureController?
    @ObservationIgnored private var pipObservation: NSKeyValueObservation?
    #else
    @ObservationIgnored private let captions = UILabel()
    @ObservationIgnored let controller = AVPlayerViewController()
    @ObservationIgnored var showOptions: (() -> Void)?
    @ObservationIgnored var showSeekPreview: (() -> Void)?
    #endif
    @ObservationIgnored var visible = false
    @ObservationIgnored var showingOptions = false
    @ObservationIgnored var restore: (() -> Void)?
    @ObservationIgnored var closedPictureInPicture: (() -> Void)?
    @ObservationIgnored private var restorationCompletion: ((Bool) -> Void)?

    override init() {
        super.init()
        #if os(iOS)
        videoView.playerLayer.videoGravity = .resizeAspect
        videoView.visibilityChanged = { [weak self] visible in
            if visible { self?.appeared() } else { self?.visible = false }
        }
        readinessObservation = videoView.playerLayer.observe(\.isReadyForDisplay, options: [.initial, .new]) { [weak self] _, _ in
            Task { @MainActor in self?.readyForDisplay = self?.videoView.playerLayer.isReadyForDisplay ?? false }
        }
        #else
        controller.delegate = self
        controller.showsPlaybackControls = true
        controller.allowsPictureInPicturePlayback = true
        readinessObservation = controller.observe(\.isReadyForDisplay, options: [.initial, .new]) { [weak self] _, _ in
            Task { @MainActor in self?.readyForDisplay = self?.controller.isReadyForDisplay ?? false }
        }
        controller.loadViewIfNeeded()
        // Keep AVKit's transport controls stable while the remote is scrubbing.
        controller.transportBarCustomMenuItems = [
            UIAction(title: "Seek with preview", image: UIImage(systemName: "film")) { [weak self] _ in self?.showSeekPreview?() },
            UIAction(title: "Playback options", image: UIImage(systemName: "ellipsis.circle")) { [weak self] _ in self?.showOptions?() },
        ]
        if let overlay = controller.contentOverlayView {
            captions.translatesAutoresizingMaskIntoConstraints = false
            captions.textColor = .white; captions.backgroundColor = UIColor.black.withAlphaComponent(0.72)
            captions.textAlignment = .center; captions.numberOfLines = 0
            captions.font = .preferredFont(forTextStyle: .title3); captions.adjustsFontForContentSizeCategory = true
            captions.isHidden = true; captions.isUserInteractionEnabled = false
            captions.isAccessibilityElement = true
            overlay.addSubview(captions)
            NSLayoutConstraint.activate([
                captions.centerXAnchor.constraint(equalTo: overlay.centerXAnchor),
                captions.widthAnchor.constraint(lessThanOrEqualTo: overlay.widthAnchor, multiplier: 0.85),
                captions.bottomAnchor.constraint(equalTo: overlay.safeAreaLayoutGuide.bottomAnchor, constant: -80)
            ])
        }
        #endif
    }

    func appeared() {
        visible = true
        restorationCompletion?(true)
        restorationCompletion = nil
    }

    func clear() {
        showCaptions("")
        #if os(iOS)
        let previous = pip
        pip = nil
        pipObservation = nil
        pictureInPicturePossible = false
        previous?.delegate = nil
        previous?.stopPictureInPicture()
        videoView.playerLayer.player = nil
        presentationMessage = nil
        #else
        controller.player = nil
        showOptions = nil
        showSeekPreview = nil
        #endif
        readyForDisplay = false
        pictureInPicture = false
        restorationCompletion?(false)
        restorationCompletion = nil
        restore = nil
    }

    func showCaptions(_ text: String) {
        guard captionText != text else { return }
        captionText = text
        #if os(tvOS)
        captions.text = text
        captions.accessibilityLabel = text
        captions.isHidden = text.isEmpty
        #endif
    }

    #if os(iOS)
    func attach(_ player: AVPlayer?) {
        guard videoView.playerLayer.player !== player else { return }
        readyForDisplay = false
        videoView.playerLayer.player = player
        if player != nil, pip == nil, AVPictureInPictureController.isPictureInPictureSupported() {
            pip = AVPictureInPictureController(playerLayer: videoView.playerLayer)
            pip?.delegate = self
            pip?.canStartPictureInPictureAutomaticallyFromInline = true
            pipObservation = pip?.observe(\.isPictureInPicturePossible, options: [.initial, .new]) { [weak self] _, _ in
                Task { @MainActor in self?.pictureInPicturePossible = self?.pip?.isPictureInPicturePossible ?? false }
            }
        }
    }

    func startPictureInPicture() {
        guard pip?.isPictureInPicturePossible == true else { return }
        presentationMessage = nil
        pip?.startPictureInPicture()
    }
    #endif

    func playerViewControllerDidStartPictureInPicture(_ playerViewController: AVPlayerViewController) { pictureInPicture = true }
    func playerViewControllerDidStopPictureInPicture(_ playerViewController: AVPlayerViewController) { pictureInPicture = false }
    func playerViewController(_ playerViewController: AVPlayerViewController,
                              restoreUserInterfaceForPictureInPictureStopWithCompletionHandler completionHandler: @escaping (Bool) -> Void) {
        restoreInterface(completionHandler)
    }

    func restoreInterface(_ completionHandler: @escaping (Bool) -> Void) {
        if visible { completionHandler(true); return }
        guard let restore else { completionHandler(false); return }
        restorationCompletion?(false)
        restorationCompletion = completionHandler
        restore()
    }
}

#if os(iOS)
extension PlayerPresentation: AVPictureInPictureControllerDelegate {
    func pictureInPictureControllerWillStartPictureInPicture(_ controller: AVPictureInPictureController) {
        guard controller === pip else { return }
        pictureInPicture = true
    }
    func pictureInPictureController(_ controller: AVPictureInPictureController, failedToStartPictureInPictureWithError error: Error) {
        guard controller === pip else { return }
        pictureInPicture = false
        presentationMessage = "Picture in Picture couldn’t start. Try again when another video or call has finished."
    }
    func pictureInPictureControllerDidStopPictureInPicture(_ controller: AVPictureInPictureController) {
        guard controller === pip else { return }
        pictureInPicture = false
        if !visible { closedPictureInPicture?() }
    }
    func pictureInPictureController(_ controller: AVPictureInPictureController,
                                    restoreUserInterfaceForPictureInPictureStopWithCompletionHandler completionHandler: @escaping (Bool) -> Void) {
        guard controller === pip else { completionHandler(false); return }
        restoreInterface(completionHandler)
    }
}
#endif
