import AVFoundation
import AVKit
import Foundation

extension PlaybackEngine {
    func install(details: PlaybackSource, compatible: Bool, at position: Double, attempt: UUID, failure: Error? = nil) async throws {
        var error = failure
        defer { if generation == attempt { recoveringNetwork = false; recoveryPosition = nil; nativeRecoveryPosition = nil } }
        while true {
            if let error {
                try check(attempt)
                guard Self.isNetworkFailure(error), networkRecoveries < 3 else { throw error }
                networkRecoveries += 1
                networkStableSince = nil
                recoveringNetwork = true
                message = "Connection interrupted. Reconnecting…"
                try await Task.sleep(for: .seconds(pow(2, Double(networkRecoveries - 1)) + Double.random(in: 0...0.5)))
            }
            try check(attempt)
            do {
                try await installSource(details: details, compatible: compatible, at: recoveryPosition ?? nativeRecoveryPosition ?? position, attempt: attempt)
                return
            } catch let next { error = next }
        }
    }

    func installSource(details: PlaybackSource, compatible: Bool, at position: Double, attempt: UUID) async throws {
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
        isPlaying = false; buffering = false
        player = nil
        usingCompatibility = compatible
        let local = try await transport.open(url: url, itemID: item.id, client: client)
        try await installPlayer(url: local, item: item, position: position, attempt: attempt)
    }

    func installPlayer(url: URL, item: MediaItem, position: Double, attempt: UUID) async throws {
        try check(attempt)
        try AVAudioSession.sharedInstance().setCategory(.playback, mode: item.isAudio ? .default : .moviePlayback)
        try AVAudioSession.sharedInstance().setActive(true)
        let retainedPosition = recoveryPosition ?? nativeRecoveryPosition ?? position
        nativeRecoveryPosition = nil
        let nextItem: AVPlayerItem
        #if os(iOS)
        nextItem = url.isFileURL ? AVPlayerItem(asset: try OfflineProbe.asset(url)) : AVPlayerItem(url: url)
        #else
        nextItem = AVPlayerItem(url: url)
        #endif
        nextItem.audioTimePitchAlgorithm = .spectral
        let next = AVPlayer(playerItem: nextItem)
        // The authenticated transport is device-local. Keep video on this
        // device for screen mirroring; system audio routes remain available.
        next.allowsExternalPlayback = false
        next.automaticallyWaitsToMinimizeStalling = true
        player = next
        #if os(iOS)
        if item.kind == .video, presentation.pictureInPicture { presentation.attach(next) }
        #endif
        let intent = NativePlaybackIntent(); nativeIntent = intent
        // AVKit controls call AVPlayer directly; retain their intent while the
        // replacement is loading, before periodic progress observation starts.
        rateObservation = next.observe(\.rate, options: [.new]) { [weak next] _, change in
            guard let next, next.currentItem?.status != .failed, let rate = change.newValue else { return }
            // Record synchronously: an actor hop could arrive after the final
            // automatic resume and override a pause from native controls.
            intent.playing.withLock { $0 = rate > 0 }
        }
        jumpObserver = NotificationCenter.default.addObserver(forName: .AVPlayerItemTimeJumped, object: nextItem, queue: .main) { [weak self, weak next] _ in
            guard !intent.seeking.withLock({ $0 }) else { return }
            Task { @MainActor in
                guard let self, let next, self.generation == attempt, self.player === next, self.loading || self.recoveringNetwork else { return }
                let time = next.currentTime().seconds
                guard (try? Input.position(time)) != nil else { return }
                self.nativeRecoveryPosition = self.timeline?.sourceTime(time) ?? time
            }
        }
        let deadline = Date().addingTimeInterval(45)
        while nextItem.status == .unknown {
            try await Task.sleep(for: .milliseconds(100))
            try check(attempt)
            guard Date() < deadline else { throw URLError(.timedOut) }
        }
        if nextItem.status == .failed {
            let failure = await transport.failure()
            try check(attempt)
            throw failure ?? nextItem.error ?? ClientError.unavailable
        }
        let actual = nextItem.duration.seconds
        if duration == 0, actual.isFinite, actual > 0 {
            let validated = try MediaTimeline(sourceDuration: actual, duration: actual)
            duration = actual
            timeline = validated
        }
        try await loadTracks(nextItem, attempt: attempt)
        try await applyPreferences(preferences)
        try check(attempt)
        var target = recoveryPosition ?? nativeRecoveryPosition ?? retainedPosition
        while true {
            try await seekPlayer(to: min(target, duration > 0 ? duration : target))
            try check(attempt)
            guard let requested = recoveryPosition ?? nativeRecoveryPosition, abs(requested - target) >= 0.01 else { break }
            target = requested
        }
        try check(attempt)
        timeObserver = next.addPeriodicTimeObserver(forInterval: CMTime(seconds: 0.25, preferredTimescale: 600), queue: .main) { [weak self] time in
            Task { @MainActor in self?.tick(time: time, attempt: attempt) }
        }
        if let endObserver { NotificationCenter.default.removeObserver(endObserver) }
        endObserver = NotificationCenter.default.addObserver(forName: .AVPlayerItemDidPlayToEndTime, object: player?.currentItem, queue: .main) { [weak self] _ in
            Task { @MainActor in guard let self, self.generation == attempt else { return }; await self.didEnd() }
        }
        message = nil
    }

    func beginMonitoring(attempt: UUID) {
        monitoring?.cancel()
        monitoring = Task { [weak self] in
            while !Task.isCancelled {
                do { try await Task.sleep(for: .seconds(1)) } catch { return }
                guard let self, self.generation == attempt, let item = self.player?.currentItem else { return }
                if item.status == .failed {
                    let failure = await self.transport.failure() ?? item.error
                    guard self.generation == attempt else { return }
                    guard let source = self.source else { self.message = Self.playbackMessage(failure); return }
                    do {
                        if Self.isNetworkFailure(failure) {
                            try await self.install(details: source, compatible: self.usingCompatibility, at: self.seconds, attempt: attempt, failure: failure)
                        } else if !self.usingCompatibility, source.compatible != nil, Self.isFormatFailure(failure) {
                            try await self.install(details: source, compatible: true, at: self.seconds, attempt: attempt)
                        } else { self.message = Self.playbackMessage(failure); return }
                        try self.check(attempt)
                        if self.nativeIntent.playing.withLock({ $0 }) ?? self.wantsPlayback { self.player?.playImmediately(atRate: Float(self.preferences.rate)) }
                    } catch {
                        if self.generation == attempt, !(error is CancellationError) { self.message = Self.playbackMessage(error) }
                        return
                    }
                }
                if let deadline = self.sleepDeadline, Date() >= deadline { self.pause(); self.sleepDeadline = nil }
            }
        }

    }

    func tick(time: CMTime, attempt: UUID) {
        guard generation == attempt, (try? Input.position(time.seconds)) != nil else { return }
        seconds = timeline?.sourceTime(time.seconds) ?? max(0, time.seconds)
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
            preferences.rate = Double(rate)
        }
        if wasPlaying, player?.timeControlStatus == .paused { saveProgress(watched: false) }
        presentation.showCaptions(subtitleDocument?.text(at: seconds) ?? "")
        if Int(seconds) != lastNowPlayingSecond {
            lastNowPlayingSecond = Int(seconds)
            nowPlaying.update(seconds: seconds, duration: duration, rate: isPlaying ? Double(player?.rate ?? 0) : 0)
        }
        if let marker = source?.markers.first(where: { source?.autoSkip.contains($0.type) == true && seconds >= $0.start && seconds < $0.end }) {
            Task { try? await seek(to: marker.end) }
        }
        if isPlaying, abs(seconds - lastSaved) >= 15 { saveProgress(watched: false) }
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
        if let jumpObserver { NotificationCenter.default.removeObserver(jumpObserver); self.jumpObserver = nil }
        if let timeObserver { player?.removeTimeObserver(timeObserver); self.timeObserver = nil }
    }
    static func isNetworkFailure(_ error: Error?, depth: Int = 0) -> Bool {
        guard depth < 8, let error else { return false }
        if let client = error as? ClientError {
            switch client {
            case .unavailable: return true
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
        // Vorbis files can surface a parser rejection through this wrapped load error.
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
