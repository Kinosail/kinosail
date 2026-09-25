import Foundation
import Observation
import WatchConnectivity

@MainActor @Observable
final class WatchRemoteSession: NSObject, WCSessionDelegate {
    private(set) var players: [WatchPlayerState] = []
    private(set) var selectedID: String?
    private(set) var busy = false
    private(set) var message: String?
    private var started = false

    var selected: WatchPlayerState? { players.first(where: { $0.id == selectedID }) }

    func start() {
        guard !started else { return }
        guard WCSession.isSupported() else {
            message = "Pair this Watch with an iPhone to control playback."
            return
        }
        started = true
        message = "Connecting to paired iPhone…"
        WCSession.default.delegate = self
        WCSession.default.activate()
    }

    func select(_ id: String) {
        guard players.contains(where: { $0.id == id }) else { return }
        selectedID = id
    }

    func refresh() async {
        await send(WatchRemoteRequest(target: nil, command: nil, position: nil))
    }

    func command(_ action: String, position: Double? = nil) async {
        guard let selected, selected.active else { return }
        await send(WatchRemoteRequest(target: selected.id, command: action, position: position))
    }

    private func send(_ request: WatchRemoteRequest) async {
        guard !busy, WCSession.default.activationState == .activated else { return }
        busy = true
        defer { busy = false }
        do {
            let payload = try request.data()
            let response: Data = try await withCheckedThrowingContinuation { continuation in
                WCSession.default.sendMessageData(payload, replyHandler: { continuation.resume(returning: $0) },
                                                  errorHandler: { continuation.resume(throwing: $0) })
            }
            let reply = try WatchRemoteReply.parse(response)
            players = reply.players
            selectedID = players.selectedID(after: selectedID)
            let unavailable = selectedID != nil && selected == nil
            message = reply.message ?? (unavailable ? "Selected player unavailable. Choose another player."
                                       : reply.accepted ? nil : "The command could not be sent.")
        } catch {
            players = []
            message = "Connect the paired iPhone to control playback."
        }
    }

    nonisolated func session(_ session: WCSession, activationDidCompleteWith activationState: WCSessionActivationState, error: Error?) {
        if activationState == .activated { Task { @MainActor [weak self] in await self?.refresh() } }
    }
}
