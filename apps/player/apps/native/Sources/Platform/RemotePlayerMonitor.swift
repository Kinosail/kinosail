#if os(tvOS)
import Foundation

@MainActor
final class RemotePlayerMonitor {
    private var id = UUID().uuidString.lowercased()
    private var profileKey: String?

    func monitor(session: AppSession) async {
        while !Task.isCancelled {
            if profileKey != session.profileKey {
                profileKey = session.profileKey
                id = UUID().uuidString.lowercased()
            }
            if let client = session.client {
                do {
                    let state = currentState(session.player)
                    if let command = try await client.updateRemotePlayer(id: id, state: state) {
                        try await apply(command, to: session.player)
                    }
                } catch is CancellationError { return }
                catch { /* Reachability is reflected by the Server's 30-second player expiry. */ }
            }
            do { try await Task.sleep(for: .seconds(3)) } catch { return }
        }
    }

    private func currentState(_ player: PlaybackCoordinator) -> WatchPlayerState {
        guard let item = player.currentItem, player.player != nil else {
            return WatchPlayerState(id: id, device: "Apple TV", title: "", subtitle: "", itemID: "",
                                    position: 0, duration: 0, playing: false, audio: false)
        }
        let duration = max(0, player.duration)
        return WatchPlayerState(id: id, device: "Apple TV", title: item.title, subtitle: item.artist,
                                itemID: item.id, position: min(duration, max(0, player.seconds)), duration: duration,
                                playing: player.isPlaying, audio: item.isAudio)
    }

    private func apply(_ command: RemotePlayerCommand, to player: PlaybackCoordinator) async throws {
        guard player.currentItem?.id == command.itemID, player.player != nil else { return }
        switch command.action {
        case "play": player.resume()
        case "pause": player.pause()
        case "backward": try await player.seek(to: max(0, player.seconds - 15))
        case "forward": try await player.seek(to: min(player.duration, player.seconds + 30))
        case "previous": if player.currentItem?.isAudio == true { try await player.previousTrack() }
        case "next": if player.currentItem?.isAudio == true { try await player.nextTrack() }
        case "seek": if let position = command.position, position <= player.duration { try await player.seek(to: position) }
        default: break
        }
    }
}
#endif
