import AVKit
import Observation

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
    @ObservationIgnored var close: (() -> Void)?
    @ObservationIgnored var userNavigated: ((AVPlayer, Double) -> Void)?
    @ObservationIgnored private var closing: Task<Void, Never>?
    @ObservationIgnored private var isolatedRecognizers: [(UIGestureRecognizer, touches: [NSNumber], presses: [NSNumber])] = []
    @ObservationIgnored private var remoteTouchesIsolated = false
    @ObservationIgnored weak var matchedDisplay: AVDisplayManager?
    var displayManager: AVDisplayManager? {
        (controller.viewIfLoaded?.window ?? UIApplication.shared.connectedScenes.lazy.compactMap { ($0 as? UIWindowScene)?.keyWindow }.first)?.avDisplayManager
    }
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
        controller.appliesPreferredDisplayCriteriaAutomatically = false
        readinessObservation = controller.observe(\.isReadyForDisplay, options: [.initial, .new]) { [weak self] _, _ in
            Task { @MainActor in self?.readyForDisplay = self?.controller.isReadyForDisplay ?? false }
        }
        controller.loadViewIfNeeded()
        controller.transportBarCustomMenuItems = [
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
        restoreRemoteTouches()
        controller.player = nil
        controller.allowedSubtitleOptionLanguages = nil
        showOptions = nil
        close = nil
        userNavigated = nil
        closing?.cancel()
        closing = nil
        #endif
        readyForDisplay = false
        pictureInPicture = false
        restorationCompletion?(false)
        restorationCompletion = nil
        restore = nil
    }

    #if os(tvOS)
    func playerViewController(_ playerViewController: AVPlayerViewController,
                              timeToSeekAfterUserNavigatedFrom oldTime: CMTime, to targetTime: CMTime) -> CMTime {
        if playerViewController === controller, let player = controller.player {
            userNavigated?(player, targetTime.seconds)
        }
        return targetTime
    }

    func playerViewControllerShouldDismiss(_ playerViewController: AVPlayerViewController) -> Bool {
        guard playerViewController === controller else { return false }
        requestClose()
        return false
    }

    func requestClose() {
        guard closing == nil, close != nil else { return }
        closing = Task { @MainActor [weak self] in
            guard !Task.isCancelled else { return }
            let close = self?.close
            self?.close = nil
            self?.closing = nil
            close?()
        }
    }

    func isolateRemoteTouches() {
        guard !remoteTouchesIsolated, controller.view.window != nil else { return }
        remoteTouchesIsolated = true
        let indirect = NSNumber(value: UITouch.TouchType.indirect.rawValue)
        let select = NSNumber(value: UIPress.PressType.select.rawValue)
        var view = controller.view.superview
        while let current = view, !(current is UIWindow) {
            for recognizer in current.gestureRecognizers ?? []
            where recognizer.allowedTouchTypes.contains(indirect) || recognizer.allowedPressTypes.contains(select) {
                isolatedRecognizers.append((recognizer, recognizer.allowedTouchTypes, recognizer.allowedPressTypes))
                recognizer.allowedTouchTypes = recognizer.allowedTouchTypes.filter { $0 != indirect }
                recognizer.allowedPressTypes = recognizer.allowedPressTypes.filter { $0 != select }
            }
            view = current.superview
        }
    }

    func restoreRemoteTouches() {
        for (recognizer, touches, presses) in isolatedRecognizers {
            recognizer.allowedTouchTypes = touches
            recognizer.allowedPressTypes = presses
        }
        isolatedRecognizers = []
        remoteTouchesIsolated = false
    }

    func limitSubtitleLanguages(to language: String?) {
        controller.allowedSubtitleOptionLanguages = language.map { [$0] }
    }
    #endif

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

    func stopPictureInPicture() { pip?.stopPictureInPicture() }
    #endif

    #if os(tvOS)
    func resetDisplay() {
        DisplayMatching.reset(matchedDisplay)
        matchedDisplay = nil
    }
    #endif

    func playerViewControllerDidStartPictureInPicture(_ playerViewController: AVPlayerViewController) { pictureInPicture = true }
    func playerViewControllerDidStopPictureInPicture(_ playerViewController: AVPlayerViewController) {
        pictureInPicture = false
        #if os(tvOS)
        if !visible { resetDisplay() }
        #endif
    }
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
