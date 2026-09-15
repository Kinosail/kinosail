import Foundation
import MediaPlayer

@MainActor
final class NowPlayingController {
    private var handlers: [(MPRemoteCommand, Any)] = []
    private var metadata: [String: Any] = [:]

    func activate(item: MediaItem, coordinator: PlaybackCoordinator) {
        deactivate()
        metadata = [MPMediaItemPropertyTitle: item.title, MPMediaItemPropertyArtist: item.artist,
                    MPMediaItemPropertyAlbumTitle: item.album, MPNowPlayingInfoPropertyIsLiveStream: false,
                    MPNowPlayingInfoPropertyMediaType: item.isAudio ? MPNowPlayingInfoMediaType.audio.rawValue : MPNowPlayingInfoMediaType.video.rawValue]
        let center = MPRemoteCommandCenter.shared()
        add(center.playCommand) { [weak coordinator] _ in Task { @MainActor in coordinator?.resume() }; return .success }
        add(center.pauseCommand) { [weak coordinator] _ in Task { @MainActor in coordinator?.pause() }; return .success }
        add(center.togglePlayPauseCommand) { [weak coordinator] _ in Task { @MainActor in coordinator?.togglePlayback() }; return .success }
        add(center.changePlaybackPositionCommand) { [weak coordinator] event in
            guard let position = (event as? MPChangePlaybackPositionCommandEvent)?.positionTime, position.isFinite, (0...31_536_000).contains(position) else { return .commandFailed }
            Task { @MainActor in try? await coordinator?.seek(to: position) }
            return .success
        }
        for (command, interval) in [(center.skipBackwardCommand, -15.0), (center.skipForwardCommand, 30.0)] {
            command.preferredIntervals = [NSNumber(value: abs(interval))]
            add(command) { [weak coordinator] _ in
                Task { @MainActor in
                    guard let coordinator else { return }
                    try? await coordinator.seek(to: min(coordinator.duration, max(0, coordinator.seconds + interval)))
                }
                return .success
            }
        }
        if item.kind == .music {
            add(center.nextTrackCommand) { [weak coordinator] _ in Task { @MainActor in try? await coordinator?.nextTrack() }; return .success }
            add(center.previousTrackCommand) { [weak coordinator] _ in Task { @MainActor in try? await coordinator?.previousTrack() }; return .success }
        }
        update(seconds: 0, duration: 0, rate: 0)
    }

    func update(seconds: Double, duration: Double, rate: Double) {
        guard !metadata.isEmpty else { return }
        metadata[MPNowPlayingInfoPropertyElapsedPlaybackTime] = seconds
        metadata[MPMediaItemPropertyPlaybackDuration] = duration
        metadata[MPNowPlayingInfoPropertyPlaybackRate] = rate
        MPNowPlayingInfoCenter.default().nowPlayingInfo = metadata
    }

    private func add(_ command: MPRemoteCommand, handler: @escaping @Sendable (MPRemoteCommandEvent) -> MPRemoteCommandHandlerStatus) {
        command.isEnabled = true
        handlers.append((command, command.addTarget(handler: handler)))
    }

    func deactivate() {
        handlers.forEach { command, token in command.removeTarget(token); command.isEnabled = false }
        handlers.removeAll()
        metadata.removeAll()
        MPNowPlayingInfoCenter.default().nowPlayingInfo = nil
    }
}
