#if os(tvOS)
import AVFoundation
import UIKit
import Testing
@testable import KinosailPlayer

@MainActor final class CompletionScrubObservation {
    private let start = ProcessInfo.processInfo.systemUptime
    private var rows: [[String: Any]] = []
    private var endpointCount = 0

    func endpoint(coordinator: PlaybackCoordinator, ended: AVPlayerItem, window: UIWindow, superseded: Bool) {
        guard endpointCount < 4 else { return }
        endpointCount += 1
        let player = coordinator.player
        rows.append(["phase": "endpoint-notification", "elapsedMs": (ProcessInfo.processInfo.systemUptime - start) * 1000,
            "alreadySuperseded": superseded, "loading": coordinator.loading, "playerAvailable": player != nil,
            "currentItemMatches": player.map { ended === $0.currentItem } ?? false,
            "attachedPlayerMatches": player.map { coordinator.presentation.controller.player === $0 } ?? false,
            "ownedWindowMatches": coordinator.presentation.controller.viewIfLoaded?.window === window])
    }

    func observe(_ phase: String, coordinator: PlaybackCoordinator, completions: Int,
                 native: Double? = nil, source: Double? = nil) {
        guard rows.count < 7 else { return }
        rows.append(["phase": phase, "elapsedMs": (ProcessInfo.processInfo.systemUptime - start) * 1000,
            "nativeSeconds": boundedPosition(native ?? coordinator.player?.currentTime().seconds),
            "sourceSeconds": boundedPosition(source ?? coordinator.seconds),
            "rate": boundedPosition(coordinator.player.map { Double($0.rate) }),
            "timeControlStatus": coordinator.player?.timeControlStatus.rawValue ?? -1,
            "loading": coordinator.loading, "completed": coordinator.completed, "completionCount": completions])
    }

    func record() {
        if let data = try? JSONSerialization.data(withJSONObject: rows, options: [.sortedKeys]) {
            Attachment.record(data, named: "native-scrub-startup.json")
            print("NATIVE_SCRUB_STARTUP " + String(decoding: data, as: UTF8.self))
        }
    }

    private func boundedPosition(_ value: Double?) -> Any {
        guard let value, value.isFinite, abs(value) <= 3600 else { return NSNull() }
        return value
    }
}

#endif
