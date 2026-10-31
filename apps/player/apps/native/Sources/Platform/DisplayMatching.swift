import AVFoundation
import AVKit
import OSLog

#if os(tvOS)
private let displayLog = Logger(subsystem: "com.kinosail.player", category: "display-matching")

@MainActor
protocol DisplayModeSwitching: AnyObject {
    var isDisplayCriteriaMatchingEnabled: Bool { get }
    var isDisplayModeSwitchInProgress: Bool { get }
    var preferredDisplayCriteria: AVDisplayCriteria? { get set }
}

extension AVDisplayManager: DisplayModeSwitching {}

enum DisplayMatching {
    enum Outcome: String { case applied, disabled, unavailable, timedOut = "timed-out" }

    @MainActor
    static func match(_ criteria: AVDisplayCriteria?, on display: (any DisplayModeSwitching)?, timeout: Duration = .seconds(6)) async -> Outcome {
        guard let display, let criteria else { return .unavailable }
        guard display.isDisplayCriteriaMatchingEnabled else { return .disabled }
        display.preferredDisplayCriteria = criteria
        let clock = ContinuousClock()
        let deadline = clock.now.advanced(by: timeout)
        try? await Task.sleep(for: .milliseconds(150))
        while display.isDisplayModeSwitchInProgress {
            guard clock.now < deadline, !Task.isCancelled else { return .timedOut }
            try? await Task.sleep(for: .milliseconds(100))
        }
        return .applied
    }

    @MainActor
    static func reset(_ display: (any DisplayModeSwitching)?) {
        display?.preferredDisplayCriteria = nil
    }
}
#endif

extension PlaybackEngine {
    func matchDisplay(for item: MediaItem, playerItem: AVPlayerItem, attempt: UUID, installation: UUID) async throws {
        #if os(tvOS)
        guard item.kind == .video else { return }
        let criteria = try? await playerItem.asset.load(.preferredDisplayCriteria)
        try check(attempt, installation: installation)
        let display = presentation.displayManager
        let outcome = await DisplayMatching.match(criteria, on: display)
        if outcome == .applied || outcome == .timedOut { presentation.matchedDisplay = display }
        displayLog.info("operation=display-match outcome=\(outcome.rawValue, privacy: .public) session=\(attempt.uuidString, privacy: .public)")
        try check(attempt, installation: installation)
        #endif
    }
}
