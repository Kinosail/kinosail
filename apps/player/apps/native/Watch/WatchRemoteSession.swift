import Foundation
import Observation
import OSLog
import WatchConnectivity

@MainActor @Observable
final class WatchRemoteSession: NSObject, WCSessionDelegate {
    enum Availability { case connecting, ready, unavailable }
    private(set) var availability = Availability.connecting
    @ObservationIgnored private let log = Logger(subsystem: "com.kinosail.player", category: "watch-remote")
    @ObservationIgnored private let connectionID = UUID().uuidString
    private(set) var players: [WatchPlayerState] = []
    private(set) var selectedID: String?
    private(set) var busy = false
    private(set) var message: String?
    private var started = false

    var selected: WatchPlayerState? { players.first(where: { $0.id == selectedID }) }

    func start() {
        guard !started else { return }
        guard WCSession.isSupported() else {
            unavailable(operation: "activate", failure: "unsupported",
                        message: "Pair this Watch with an iPhone to control playback.")
            return
        }
        started = true
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
        let requestID = UUID().uuidString
        defer { busy = false }
        do {
            let payload = try request.data()
            let response = try await Self.exchange(payload)
            let reply = try WatchRemoteReply.parse(response)
            if availability != .ready {
                let outcome = availability == .unavailable ? "recovered" : "connected"
                log.info("Watch remote state operation=refresh outcome=\(outcome, privacy: .public) session_id=\(self.connectionID, privacy: .public) request_id=\(requestID, privacy: .public)")
            }
            availability = .ready
            players = reply.players
            selectedID = players.selectedID(after: selectedID)
            let unavailable = selectedID != nil && selected == nil
            message = reply.message ?? (unavailable ? "Selected player unavailable. Choose another player."
                                       : reply.accepted ? nil : "The command could not be sent.")
        } catch {
            unavailable(operation: "exchange", failure: error is WatchRemoteError || error is DecodingError ? "invalid_message" : "transport", requestID: requestID)
        }
    }

    private func unavailable(operation: String, failure: String, requestID: String = "none",
                             message: String = "Connect the paired iPhone to control playback.") {
        if availability != .unavailable {
            log.warning("Watch remote unavailable operation=\(operation, privacy: .public) failure=\(failure, privacy: .public) session_id=\(self.connectionID, privacy: .public) request_id=\(requestID, privacy: .public)")
        }
        availability = .unavailable
        players = []
        self.message = message
    }

    nonisolated private static func exchange(_ payload: Data) async throws -> Data {
        try await withCheckedThrowingContinuation { continuation in
            WCSession.default.sendMessageData(payload, replyHandler: { continuation.resume(returning: $0) },
                                              errorHandler: { continuation.resume(throwing: $0) })
        }
    }

    nonisolated func session(_ session: WCSession, activationDidCompleteWith activationState: WCSessionActivationState, error: Error?) {
        Task { @MainActor [weak self] in
            guard let self else { return }
            guard activationState == .activated else {
                unavailable(operation: "activate", failure: "activation")
                return
            }
            await refresh()
        }
    }
}
