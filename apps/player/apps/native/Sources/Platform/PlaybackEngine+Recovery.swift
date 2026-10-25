import AVFoundation
import AVKit
import Foundation
import OSLog
import Synchronization

private let recoveryLog = Logger(subsystem: "com.kinosail.player", category: "playback-recovery")

extension PlaybackEngine {
    func install(details: PlaybackSource, compatible: Bool, at position: Double, attempt: UUID, installation: UUID, failure: Error? = nil) async throws {
        try check(attempt, installation: installation)
        loading = true
        var error = failure
        defer { if generation == attempt, self.installation == installation { loading = false; recoveringNetwork = false; recoveryPosition = nil; nativeRecoveryPosition = nil } }
        while true {
            if let error {
                try check(attempt, installation: installation)
                guard Self.isNetworkFailure(error), networkRecoveries < 8 else { throw error }
                networkRecoveries += 1
                networkStableSince = nil
                recoveringNetwork = true
                message = "Connection interrupted. Reconnecting…"
                try await Task.sleep(for: .seconds(min(8, pow(2, Double(networkRecoveries - 1))) + Double.random(in: 0...0.5)))
            }
            try check(attempt, installation: installation)
            do {
                try await installSource(details: details, compatible: compatible, at: recoveryPosition ?? nativeRecoveryPosition ?? position, attempt: attempt, installation: installation)
                return
            } catch let next {
                try check(attempt, installation: installation)
                error = next
            }
        }
    }

    func installSource(details: PlaybackSource, compatible: Bool, at position: Double, attempt: UUID, installation: UUID) async throws {
        try check(attempt, installation: installation)
        guard let client, let item = currentItem else { throw CancellationError() }
        let url: URL
        if compatible {
            guard let fallback = details.compatible else { throw ClientError.invalidInput("The Server has no compatible stream for this title.") }
            url = fallback.url
            timeline = fallback.timeline
        } else {
            guard let direct = details.direct else { throw ClientError.invalidResponse }
            url = direct
            timeline = try MediaTimeline(sourceDuration: details.duration, duration: details.duration)
        }
        wantsPlayback = nativeIntent.playing.withLock { $0 } ?? wantsPlayback
        removeTimeObserver()
        player?.pause()
        isPlaying = false; buffering = false; bufferedRanges = []
        player = nil
        usingCompatibility = compatible
        let local = try await transport.open(url: url, itemID: item.id, client: client)
        try await installPlayer(url: local, item: item, position: position, attempt: attempt, installation: installation)
    }

    func installPlayer(url: URL, item: MediaItem, position: Double, attempt: UUID, installation: UUID) async throws {
        try check(attempt, installation: installation)
        bufferedRanges = []
        try await PlaybackAudioSession.activate(audio: item.isAudio, session: attempt)
        try check(attempt, installation: installation)
        let retainedPosition = recoveryPosition ?? nativeRecoveryPosition ?? position
        nativeRecoveryPosition = nil
        let nextItem: AVPlayerItem
        #if os(iOS)
        nextItem = url.isFileURL ? AVPlayerItem(asset: try OfflineProbe.asset(url)) : AVPlayerItem(url: url)
        #else
        nextItem = AVPlayerItem(url: url)
        nextItem.externalMetadata = PlaybackExternalMetadata.items(for: item)
        publishArtwork(for: item, on: nextItem)
        #endif
        nextItem.audioTimePitchAlgorithm = .spectral
        let next = AVPlayer(playerItem: nextItem)
        next.allowsExternalPlayback = false
        next.automaticallyWaitsToMinimizeStalling = true
        player = next
        #if os(iOS)
        if item.kind == .video, presentation.pictureInPicture { presentation.attach(next) }
        #endif
        let intent = NativePlaybackIntent(); nativeIntent = intent
        rateObservation = next.observe(\.rate, options: [.new]) { [weak next] _, change in
            guard let next, next.currentItem?.status != .failed, let rate = change.newValue else { return }
            intent.playing.withLock { $0 = rate > 0 }
        }
        #if os(tvOS)
        presentation.userNavigated = { [weak self, weak next] active, time in
            guard let self, let next, active === next, self.player === next, self.generation == attempt, self.installation == installation,
                  self.loading || self.recoveringNetwork, (try? Input.position(time)) != nil else { return }
            self.nativeRecoveryPosition = self.timeline?.sourceTime(time) ?? time
        }
        #endif
        try await awaitReadiness(of: nextItem, attempt: attempt, installation: installation)
        if nextItem.status == .failed {
            let failure = await transport.failure()
            try check(attempt, installation: installation)
            throw failure ?? nextItem.error ?? ClientError.unavailable
        }
        let (mode, reason) = usingCompatibility ? (source?.compatible?.mode ?? "unknown", source?.compatible?.reason ?? "unknown") : ("direct", "compatible")
        recoveryLog.info("operation=playback-install outcome=ready compatible=\(self.usingCompatibility, privacy: .public) mode=\(mode, privacy: .public) reason=\(reason, privacy: .public) live=\(nextItem.duration.isIndefinite, privacy: .public) session=\(attempt.uuidString, privacy: .public)")
        let actual = nextItem.duration.seconds
        if duration == 0, actual.isFinite, actual > 0 {
            let validated = try MediaTimeline(sourceDuration: actual, duration: actual)
            duration = actual
            timeline = validated
        }
        try await loadTracks(nextItem, attempt: attempt, installation: installation)
        try check(attempt, installation: installation)
        try await applyPreferences(preferences)
        try check(attempt, installation: installation)
        if let variantObserver { NotificationCenter.default.removeObserver(variantObserver) }
        variantObserver = NotificationCenter.default.addObserver(forName: AVPlayerItem.newAccessLogEntryNotification, object: nextItem, queue: .main) { [weak nextItem] _ in
            guard let event = nextItem?.accessLog()?.events.last, let size = nextItem?.presentationSize else { return }
            let indicated = event.indicatedBitrate.isFinite ? Int(max(0, event.indicatedBitrate)) : 0
            let observed = event.observedBitrate.isFinite ? Int(max(0, event.observedBitrate)) : 0
            recoveryLog.info("operation=playback-variant indicatedBitrate=\(indicated, privacy: .public) observedBitrate=\(observed, privacy: .public) width=\(Int(size.width), privacy: .public) height=\(Int(size.height), privacy: .public) session=\(attempt.uuidString, privacy: .public)")
        }
        let completion = Mutex((restoring: true, ended: false))
        if let endObserver { NotificationCenter.default.removeObserver(endObserver) }
        endObserver = NotificationCenter.default.addObserver(forName: .AVPlayerItemDidPlayToEndTime, object: nextItem, queue: nil) { [weak self, weak next, weak nextItem] _ in
            let deferred = completion.withLock { state in
                if state.restoring { state.ended = true; return true }
                return false
            }
            guard !deferred else { return }
            Task { @MainActor in
                guard let self, let next, let nextItem else { return }
                await self.didEnd(attempt: attempt, player: next, nativeItem: nextItem)
            }
        }
        var target = recoveryPosition ?? nativeRecoveryPosition ?? retainedPosition
        while true {
            try check(attempt, installation: installation)
            try await seekPlayer(to: min(target, duration > 0 ? duration : target))
            try check(attempt, installation: installation)
            guard player === next, next.currentItem === nextItem else { throw CancellationError() }
            guard let requested = recoveryPosition ?? nativeRecoveryPosition, abs(requested - target) >= 0.01 else { break }
            completion.withLock { $0.ended = false }
            target = requested
        }
        try check(attempt, installation: installation)
        timeObserver = next.addPeriodicTimeObserver(forInterval: CMTime(seconds: 0.25, preferredTimescale: 600), queue: .main) { [weak self, weak next, weak nextItem] time in
            Task { @MainActor in
                guard let self, let next, let nextItem, self.player === next,
                      self.installation == installation, next.currentItem === nextItem,
                      nextItem.status == .readyToPlay else { return }
                self.tick(time: time, attempt: attempt)
            }
        }
        message = nil
        let ended = completion.withLock { state in state.restoring = false; return state.ended }
        if ended { await didEnd(attempt: attempt, player: next, nativeItem: nextItem, restored: true) }
    }

    func beginMonitoring(attempt: UUID) {
        monitoring?.cancel()
        let initial = installation
        monitoring = Task { [weak self] in
            var installation = initial
            while !Task.isCancelled {
                do { try await Task.sleep(for: .seconds(1)) } catch { return }
                guard let self, self.generation == attempt, self.installation == installation, let item = self.player?.currentItem else { return }
                self.refreshBufferedRanges()
                if item.status == .failed {
                    let failure = await self.transport.failure() ?? item.error
                    guard self.generation == attempt, self.installation == installation, self.player?.currentItem === item else { return }
                    guard let source = self.source else { self.message = Self.playbackMessage(failure); return }
                    let failureClass = Self.isNetworkFailure(failure) ? "network" : Self.isFormatFailure(failure) ? "format" : "terminal"
                    let code = (failure as NSError?)?.code ?? 0
                    recoveryLog.warning("operation=playback-recovery failure_class=\(failureClass, privacy: .public) code=\(code, privacy: .public) compatible=\(self.usingCompatibility, privacy: .public) recoveries=\(self.networkRecoveries, privacy: .public) session=\(attempt.uuidString, privacy: .public)")
                    do {
                        if Self.isNetworkFailure(failure) {
                            installation = self.beginInstallation()
                            try await self.install(details: source, compatible: self.usingCompatibility, at: self.seconds, attempt: attempt, installation: installation, failure: failure)
                        } else if !self.usingCompatibility, source.compatible != nil, Self.isFormatFailure(failure) {
                            installation = self.beginInstallation()
                            try await self.install(details: source, compatible: true, at: self.seconds, attempt: attempt, installation: installation)
                        } else { self.message = Self.playbackMessage(failure); return }
                        try self.check(attempt, installation: installation)
                        guard !self.completed else { return }
                        if self.nativeIntent.playing.withLock({ $0 }) ?? self.wantsPlayback { self.player?.play() }
                    } catch {
                        if self.generation == attempt, self.installation == installation, !(error is CancellationError) { self.message = Self.playbackMessage(error) }
                        return
                    }
                }
                if let deadline = self.sleepDeadline, Date() >= deadline { self.pause(); self.sleepDeadline = nil }
            }
        }

    }

    func tick(time: CMTime, attempt: UUID) {
        guard generation == attempt, (try? Input.position(time.seconds)) != nil else { return }
        refreshBufferedRanges()
        seconds = timeline?.sourceTime(time.seconds) ?? max(0, time.seconds)
        committedPosition = seconds
        if !loading, !recoveringNetwork, player?.currentItem?.status == .readyToPlay {
            wantsPlayback = player?.timeControlStatus != .paused
        }
        if player?.timeControlStatus == .playing {
            if networkStableSince == nil { networkStableSince = Date() }
            if let stable = networkStableSince, Date().timeIntervalSince(stable) >= 30 { networkRecoveries = 0 }
        } else { networkStableSince = nil }
        let wasPlaying = isPlaying
        isPlaying = player?.timeControlStatus == .playing
        buffering = player?.timeControlStatus == .waitingToPlayAtSpecifiedRate
        if let rate = player?.defaultRate, rate.isFinite, (0.5...3).contains(rate) {
            playbackRate = Double(rate)
            if preferences.rate != Double(rate) {
                preferences.rate = Double(rate)
                rememberChoices { $0.rate = Double(rate) }
            }
        }
        if let audioID = selectedAudioTrackID, audioID != observedAudioTrackID { rememberAudioSelection(id: audioID) }
        let subtitleID = selectedSubtitleTrackID
        if subtitleID != observedSubtitleTrackID { rememberSubtitleSelection(id: subtitleID) }
        if wasPlaying, player?.timeControlStatus == .paused { saveProgress(watched: false) }
        presentation.showCaptions(subtitleDocument?.text(at: seconds) ?? "")
        if Int(seconds) != lastNowPlayingSecond {
            lastNowPlayingSecond = Int(seconds)
            nowPlaying.update(seconds: seconds, duration: duration, rate: isPlaying ? Double(player?.rate ?? 0) : 0)
        }
        if let marker = source?.markers.first(where: { source?.autoSkip.contains($0.type) == true && seconds >= $0.start && seconds < $0.end }) {
            Task { if marker.end >= duration - 1, source?.nextItemID != nil { await finishIntoNextEpisode() } else { try? await seek(to: marker.end) } }
        }
        if isPlaying, abs(seconds - lastSaved) >= 15 { saveProgress(watched: false) }
    }

    func refreshBufferedRanges() {
        guard let item = player?.currentItem, item.status == .readyToPlay, let timeline,
              item.loadedTimeRanges.count <= 128 else { bufferedRanges = []; return }
        bufferedRanges = timeline.bufferedRanges(for: item.loadedTimeRanges.map {
            let range = $0.timeRangeValue
            return (range.start.seconds, range.start.seconds + range.duration.seconds)
        })
    }

    func observeAudioSession() {
        let center = NotificationCenter.default
        notifications.append(center.addObserver(forName: AVAudioSession.interruptionNotification, object: nil, queue: .main) { [weak self] notification in
            let type = (notification.userInfo?[AVAudioSessionInterruptionTypeKey] as? UInt) ?? 0
            let options = (notification.userInfo?[AVAudioSessionInterruptionOptionKey] as? UInt) ?? 0
            Task { @MainActor in
                guard let self else { return }
                if type == AVAudioSession.InterruptionType.began.rawValue { self.resumeAfterInterruption = self.isPlaying; self.pause() }
                else if self.resumeAfterInterruption, AVAudioSession.InterruptionOptions(rawValue: options).contains(.shouldResume) { self.resume() }
            }
        })
        notifications.append(center.addObserver(forName: AVAudioSession.routeChangeNotification, object: nil, queue: .main) { [weak self] notification in
            let reason = (notification.userInfo?[AVAudioSessionRouteChangeReasonKey] as? UInt) ?? 0
            Task { @MainActor in if reason == AVAudioSession.RouteChangeReason.oldDeviceUnavailable.rawValue { self?.pause() } }
        })
    }

    func removeTimeObserver() {
        rateObservation?.invalidate(); rateObservation = nil
        #if os(tvOS)
        presentation.userNavigated = nil
        #endif
        if let timeObserver { player?.removeTimeObserver(timeObserver); self.timeObserver = nil }
    }
    static func isNetworkFailure(_ error: Error?, depth: Int = 0) -> Bool {
        guard depth < 8, let error else { return false }
        if let client = error as? ClientError {
            switch client {
            case .unavailable, .retryAfter: return true
            case .http(let code): return [408, 429, 500, 502, 503, 504].contains(code)
            default: return false
            }
        }
        let value = error as NSError
        if value.domain == NSURLErrorDomain {
            return [URLError.timedOut, .cannotFindHost, .cannotConnectToHost, .networkConnectionLost, .dnsLookupFailed, .notConnectedToInternet, .resourceUnavailable].contains(URLError.Code(rawValue: value.code))
        }
        return (value.userInfo[NSUnderlyingErrorKey] as? NSError).map { isNetworkFailure($0, depth: depth + 1) } ?? false
    }

    static func isFormatFailure(_ error: Error?, depth: Int = 0) -> Bool {
        guard depth < 8 else { return false }
        guard let error = error as NSError? else { return false }
        if error.domain == AVFoundationErrorDomain, [AVError.fileFormatNotRecognized.rawValue, AVError.fileFailedToParse.rawValue, AVError.decoderNotFound.rawValue, AVError.decodeFailed.rawValue, AVError.operationNotSupportedForAsset.rawValue].contains(error.code) { return true }
        if error.domain == AVFoundationErrorDomain, error.code == AVError.failedToLoadMediaData.rawValue,
           let cause = error.userInfo[NSUnderlyingErrorKey] as? NSError,
           cause.domain == NSOSStatusErrorDomain, cause.code == -12873 { return true }
        return (error.userInfo[NSUnderlyingErrorKey] as? NSError).map { isFormatFailure($0, depth: depth + 1) } ?? false
    }

    static func playbackMessage(_ error: Error?) -> String {
        if isNetworkFailure(error) { return "The connection is still unavailable. Your position is saved. Reopen this title when the connection returns." }
        if let error = error as? ClientError { return error.localizedDescription }
        return "This media could not be played. Check the Server or try a different title."
    }
}
