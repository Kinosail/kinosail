import AVFoundation
import Foundation
import OSLog

enum PlaybackAudioSession {
    private static let queue = DispatchQueue(label: "com.kinosail.player.audio-session")
    private static let log = Logger(subsystem: "com.kinosail.player", category: "audio-session")

    @MainActor static func activate(audio: Bool, session: UUID) async throws {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            queue.async {
                let mode = audio ? "audio" : "video"
                var phase = "category"
                do {
                    let audioSession = AVAudioSession.sharedInstance()
                    try audioSession.setCategory(.playback, mode: audio ? .default : .moviePlayback)
                    phase = "activation"
                    try audioSession.setActive(true)
                    log.debug("operation=audio-session outcome=activated mode=\(mode, privacy: .public) session=\(session.uuidString, privacy: .public)")
                    continuation.resume()
                } catch {
                    let code = (error as NSError).code
                    log.warning("operation=audio-session outcome=failed phase=\(phase, privacy: .public) mode=\(mode, privacy: .public) session=\(session.uuidString, privacy: .public) code=\(code)")
                    continuation.resume(throwing: error)
                }
            }
        }
    }
}
