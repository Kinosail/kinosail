#if os(iOS)
import Foundation
import WatchConnectivity

private struct WatchReplyHandler: @unchecked Sendable {
    // Watch Connectivity permits its reply handler to run on any queue. This value is called once.
    let send: (Data) -> Void
}

@MainActor
final class WatchRemoteBridge: NSObject, WCSessionDelegate {
    private weak var app: AppSession?
    private var cachedTV: [WatchPlayerState] = []

    func attach(_ app: AppSession) {
        self.app = app
        guard WCSession.isSupported() else { return }
        WCSession.default.delegate = self
        WCSession.default.activate()
    }

    nonisolated func session(_ session: WCSession, activationDidCompleteWith activationState: WCSessionActivationState, error: Error?) {}
    nonisolated func sessionDidBecomeInactive(_ session: WCSession) {}
    nonisolated func sessionDidDeactivate(_ session: WCSession) { session.activate() }

    nonisolated func session(_ session: WCSession, didReceiveMessageData messageData: Data, replyHandler: @escaping (Data) -> Void) {
        let reply = WatchReplyHandler(send: replyHandler)
        Task { @MainActor [weak self] in
            let data = await self?.reply(to: messageData) ?? Data()
            reply.send(data)
        }
    }

    private func reply(to data: Data) async -> Data {
        guard let app else { return Data() }
        do {
            let request = try WatchRemoteRequest.parse(data)
            var accepted = true
            var message: String?
            if let target = request.target {
                do { try await apply(request, target: target, app: app) }
                catch { accepted = false; message = "That player could not complete the command." }
            }
            if let client = app.client {
                do { cachedTV = try await client.remotePlayers() }
                catch { cachedTV = []; message = "Apple TV is unavailable. Check its Server connection." }
            } else { cachedTV = [] }
            return try WatchRemoteReply(players: [phoneState(app)] + cachedTV, accepted: accepted, message: message).data()
        } catch {
            return (try? WatchRemoteReply(players: [phoneState(app)] + cachedTV, accepted: false,
                                          message: "The watch command was invalid.").data()) ?? Data()
        }
    }

    private func phoneState(_ app: AppSession) -> WatchPlayerState {
        guard let item = app.player.currentItem, app.player.player != nil else {
            return WatchPlayerState(id: "iphone", device: "This iPhone", title: "", subtitle: "", itemID: "",
                                    position: 0, duration: 0, playing: false, audio: false)
        }
        let duration = max(0, app.player.duration)
        return WatchPlayerState(id: "iphone", device: "This iPhone", title: item.title, subtitle: item.artist,
                                itemID: item.id, position: min(duration, max(0, app.player.seconds)), duration: duration,
                                playing: app.player.isPlaying, audio: item.isAudio)
    }

    private func apply(_ request: WatchRemoteRequest, target: String, app: AppSession) async throws {
        guard let command = request.command else { throw WatchRemoteError.invalidMessage }
        if target == "iphone" {
            guard app.player.currentItem != nil, app.player.player != nil else { throw WatchRemoteError.invalidMessage }
            switch command {
            case "play": app.player.resume()
            case "pause": app.player.pause()
            case "backward": try await app.player.seek(to: max(0, app.player.seconds - 15))
            case "forward": try await app.player.seek(to: min(app.player.duration, app.player.seconds + 30))
            case "previous": guard app.player.currentItem?.isAudio == true else { throw WatchRemoteError.invalidMessage }; try await app.player.previousTrack()
            case "next": guard app.player.currentItem?.isAudio == true else { throw WatchRemoteError.invalidMessage }; try await app.player.nextTrack()
            case "seek": guard let position = request.position, position <= app.player.duration else { throw WatchRemoteError.invalidMessage }; try await app.player.seek(to: position)
            default: throw WatchRemoteError.invalidMessage
            }
            return
        }
        guard let client = app.client, let player = cachedTV.first(where: { $0.id == target && $0.active }) else { throw WatchRemoteError.invalidMessage }
        try await client.commandRemotePlayer(id: target, itemID: player.itemID, command: request)
    }
}
#endif
