#if os(iOS)
import Observation
import OSLog
import UIKit

/// Explicit video rotation belongs to its window scene, without changing the system lock.
@MainActor @Observable
final class PlaybackOrientation {
    private(set) var requesting = false
    private(set) var landscapeRequested = false
    private(set) var message: String?
    @ObservationIgnored private weak var window: UIWindow?
    @ObservationIgnored private var prior: UIInterfaceOrientationMask?
    @ObservationIgnored private var requestID: UUID?
    @ObservationIgnored private var pending: Task<Void, Never>?
    @ObservationIgnored private let log = Logger(subsystem: "com.kinosail.player", category: "orientation")

    func toggle(in window: UIWindow?) {
        guard !requesting else { return }
        if landscapeRequested { restore(); return }
        guard let window, window.traitCollection.userInterfaceIdiom == .phone,
              let scene = window.windowScene, scene.activationState == .foregroundActive,
              let current = Self.mask(scene.effectiveGeometry.interfaceOrientation) else {
            message = "Landscape playback isn’t available in this window. Try again when the player is active."
            log.warning("Playback orientation unavailable operation=landscape failure=inactive_window")
            return
        }
        self.window = window
        if prior == nil { prior = current }
        request(.landscape, entering: true)
    }

    func restore(onFailure: @escaping @MainActor (String) -> Void = { _ in }) {
        guard let prior else { return }
        request(prior, entering: false, onFailure: onFailure)
    }

    private func request(_ target: UIInterfaceOrientationMask, entering: Bool,
                         onFailure: @escaping @MainActor (String) -> Void = { _ in }) {
        pending?.cancel()
        let id = UUID()
        requestID = id
        requesting = true
        message = nil
        let operation = entering ? "landscape" : "restore"
        guard let window, let scene = window.windowScene, scene.activationState == .foregroundActive else {
            fail(id, operation: operation, code: nil, onFailure: onFailure)
            return
        }
        // UIKit compares the app and visible controller's supported orientations.
        var controller = window.rootViewController
        while let current = controller {
            current.setNeedsUpdateOfSupportedInterfaceOrientations()
            controller = current.presentedViewController
        }
        scene.requestGeometryUpdate(.iOS(interfaceOrientations: target)) { [weak self] error in
            let code = (error as NSError).code
            Task { @MainActor in self?.fail(id, operation: operation, code: code, onFailure: onFailure) }
        }
        pending = Task {
            for _ in 0..<30 {
                do { try await Task.sleep(for: .milliseconds(100)) } catch { return }
                guard requestID == id else { return }
                if let actual = Self.mask(scene.effectiveGeometry.interfaceOrientation), !target.intersection(actual).isEmpty {
                    landscapeRequested = entering
                    requesting = false
                    requestID = nil
                    pending = nil
                    if !entering { prior = nil; self.window = nil }
                    log.debug("Playback orientation confirmed operation=\(operation, privacy: .public) request_id=\(id.uuidString, privacy: .public)")
                    return
                }
            }
            fail(id, operation: operation, code: nil, onFailure: onFailure)
        }
    }

    private func fail(_ id: UUID, operation: String, code: Int?, onFailure: @MainActor (String) -> Void) {
        guard requestID == id else { return }
        pending?.cancel()
        pending = nil
        requestID = nil
        requesting = false
        let text = operation == "restore"
            ? "iOS couldn’t return to the previous orientation. Try rotating the phone when you’re ready."
            : "iOS couldn’t rotate the player. Try again when another window or call has finished."
        message = text
        log.warning("Playback orientation denied operation=\(operation, privacy: .public) request_id=\(id.uuidString, privacy: .public) code=\(code ?? 0)")
        onFailure(text)
    }

    private static func mask(_ orientation: UIInterfaceOrientation) -> UIInterfaceOrientationMask? {
        switch orientation {
        case .portrait: .portrait
        case .landscapeLeft: .landscapeLeft
        case .landscapeRight: .landscapeRight
        case .portraitUpsideDown: .portraitUpsideDown
        default: nil
        }
    }
}
#endif
