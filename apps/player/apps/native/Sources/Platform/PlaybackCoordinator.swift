import AVFoundation
import AVKit
import Observation
import Synchronization

struct PlayerTrack: Identifiable {
    let id: String
    let title: String
}

private final class NativePlaybackIntent: Sendable {
    let playing = Mutex<Bool?>(nil)
    let seeking = Mutex(false)
}

@MainActor @Observable
final class PlaybackCoordinator {
    private(set) var player: AVPlayer?
    private(set) var currentItem: MediaItem?
    private(set) var source: PlaybackSource?
    private(set) var seconds: Double = 0
    private(set) var duration: Double = 0
    private(set) var loading = false
    private(set) var isPlaying = false
    private(set) var buffering = false
    private(set) var usingCompatibility = false
    private(set) var message: String?
    private(set) var progressMessage: String?
    private(set) var audioTracks: [PlayerTrack] = []
    private(set) var subtitleTracks: [PlayerTrack] = []
    private(set) var externalCaptions = false
    private(set) var playbackRate: Double = 1
    private(set) var selectedExternalSubtitleID: String?
    private(set) var recoveringNetwork = false
    private(set) var completed = false
    private(set) var sleepDeadline: Date?
    let queue = MediaQueue()
    let presentation = PlayerPresentation()
    @ObservationIgnored var onCompleted: (@MainActor (MediaItem, String?) -> Void)?

    @ObservationIgnored private var transport = MediaTransport()
    @ObservationIgnored private var generation = UUID()
    @ObservationIgnored private var client: ServerClient?
    @ObservationIgnored private var store: ProgressSyncStore?
    @ObservationIgnored private var writer: ProgressWriter?
    @ObservationIgnored private var timeline: MediaTimeline?
    @ObservationIgnored private var preferences = PlaybackPreferences()
    @ObservationIgnored private var timeObserver: Any?
    @ObservationIgnored private var rateObservation: NSKeyValueObservation?
    @ObservationIgnored private var jumpObserver: NSObjectProtocol?
    @ObservationIgnored private var monitoring: Task<Void, Never>?
    @ObservationIgnored private var writing: Task<Void, Never>?
    @ObservationIgnored private var savingRate: Task<Void, Never>?
    @ObservationIgnored private var notifications: [NSObjectProtocol] = []
    @ObservationIgnored private var nowPlaying = NowPlayingController()
    @ObservationIgnored private var lastSaved: Double = -30
    @ObservationIgnored private var audioOptions: [AVMediaSelectionOption] = []
    @ObservationIgnored private var subtitleOptions: [AVMediaSelectionOption] = []
    @ObservationIgnored private var audioGroup: AVMediaSelectionGroup?
    @ObservationIgnored private var subtitleGroup: AVMediaSelectionGroup?
    @ObservationIgnored private var resumeAfterInterruption = false
    @ObservationIgnored private var endObserver: NSObjectProtocol?
    @ObservationIgnored private var subtitleDocument: SubtitleDocument?
    @ObservationIgnored private var subtitleGeneration = UUID()
    @ObservationIgnored private var lastNowPlayingSecond = -1
    @ObservationIgnored private var nativeIntent = NativePlaybackIntent()
    @ObservationIgnored private var wantsPlayback = false
    @ObservationIgnored private var networkRecoveries = 0
    @ObservationIgnored private var networkStableSince: Date?
    @ObservationIgnored private var recoveryPosition: Double?
    @ObservationIgnored private var nativeRecoveryPosition: Double?

    init() {
        presentation.closedPictureInPicture = { [weak self] in self?.stop() }
    }

    var selectedAudioTrackID: String? {
        guard let group = audioGroup, let option = player?.currentItem?.currentMediaSelection.selectedMediaOption(in: group),
              let index = audioOptions.firstIndex(of: option) else { return nil }
        return String(index)
    }

    var selectedSubtitleTrackID: String? {
        if let selectedExternalSubtitleID { return selectedExternalSubtitleID }
        guard let group = subtitleGroup, let option = player?.currentItem?.currentMediaSelection.selectedMediaOption(in: group),
              let index = subtitleOptions.firstIndex(of: option) else { return nil }
        return String(index)
    }

    func play(_ item: MediaItem, client: ServerClient, store: ProgressSyncStore) async throws {
        guard [.video, .music, .audiobook].contains(item.kind) else { throw ClientError.invalidInput("This title does not contain playable audio or video.") }
        stop(clearQueue: item.kind != .music)
        let attempt = generation
        self.client = client; self.store = store; currentItem = item; loading = true; wantsPlayback = true
        defer { if generation == attempt { loading = false } }
        do {
            async let playback = client.playback(itemID: item.id)
            async let savedPreferences = client.playbackPreferences(itemID: item.id)
            let details = try await playback
            let saved = try await savedPreferences
            try check(attempt)
            source = details
            preferences = saved.playback
            duration = details.duration
            writer = try ProgressWriter(itemID: item.id, expected: item.progress, client: client, store: store)
            let pending = try await store.pending().first { $0.itemID == item.id }
            try check(attempt)
            let start = pending?.progress.seconds ?? details.start
            do { try await install(details: details, compatible: details.direct == nil, at: start, attempt: attempt) }
            catch {
                try check(attempt)
                guard details.direct != nil, details.compatible != nil, Self.isFormatFailure(error) else { throw error }
                try await install(details: details, compatible: true, at: start, attempt: attempt)
            }
            try check(attempt)
            nowPlaying.activate(item: item, coordinator: self)
            beginMonitoring(attempt: attempt)
            observeAudioSession()
            if nativeIntent.playing.withLock({ $0 }) ?? wantsPlayback { player?.playImmediately(atRate: Float(preferences.rate)) }
        } catch {
            if generation == attempt, !(error is CancellationError) { message = Self.playbackMessage(error); player?.pause() }
            throw error
        }
    }

    #if os(iOS)
    func playOffline(_ item: MediaItem, file: URL, downloadID: String, client: ServerClient, store: ProgressSyncStore, preferences: PlaybackPreferences) async throws {
        _ = try Input.hex(downloadID, count: 64)
        guard [.video, .music, .audiobook].contains(item.kind) else { throw ClientError.invalidInput("This download does not contain playable media.") }
        let scope = try await client.profileScope()
        try Task.checkCancellation()
        let root = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask)[0].resolvingSymlinksInPath().appendingPathComponent("kinosail-swift-offline")
        let expected = root.appendingPathComponent(scope).appendingPathComponent(downloadID + ".media")
        guard file == expected, file.isFileURL, file.resolvingSymlinksInPath().standardizedFileURL == expected.standardizedFileURL else { throw ClientError.invalidResponse }
        let preferences = try PlaybackPreferences(preferences.json)
        stop()
        let attempt = generation
        currentItem = item; self.client = client; self.store = store; self.preferences = preferences; loading = true; wantsPlayback = true
        defer { if generation == attempt { loading = false } }
        do {
            writer = try ProgressWriter(itemID: item.id, expected: item.progress, client: client, store: store)
            let pending = try await store.pending().first { $0.itemID == item.id }
            try check(attempt)
            let position = pending?.progress.seconds ?? (item.progress.watched ? 0 : item.progress.seconds)
            try await installPlayer(url: file, item: item, position: position, attempt: attempt)
            try check(attempt)
            nowPlaying.activate(item: item, coordinator: self)
            beginMonitoring(attempt: attempt); observeAudioSession()
            if nativeIntent.playing.withLock({ $0 }) ?? wantsPlayback { player?.playImmediately(atRate: Float(preferences.rate)) }
        } catch {
            if generation == attempt, !(error is CancellationError) { message = Self.playbackMessage(error); player?.pause() }
            throw error
        }
    }
    #endif

    private func install(details: PlaybackSource, compatible: Bool, at position: Double, attempt: UUID, failure: Error? = nil) async throws {
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

    private func installSource(details: PlaybackSource, compatible: Bool, at position: Double, attempt: UUID) async throws {
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

    private func installPlayer(url: URL, item: MediaItem, position: Double, attempt: UUID) async throws {
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
                guard time.isFinite, time >= 0 else { return }
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
            duration = actual
            timeline = try MediaTimeline(sourceDuration: actual, duration: actual)
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

    private func beginMonitoring(attempt: UUID) {
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

    private func tick(time: CMTime, attempt: UUID) {
        guard generation == attempt, time.seconds.isFinite else { return }
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

    func seek(to seconds: Double) async throws {
        try Input.position(seconds)
        guard duration == 0 || seconds <= duration + 1 else { throw ClientError.invalidInput("The playback position is unavailable.") }
        if recoveringNetwork || loading { recoveryPosition = seconds; self.seconds = seconds; return }
        try await seekPlayer(to: seconds)
    }

    private func seekPlayer(to seconds: Double) async throws {
        try Input.position(seconds)
        guard duration == 0 || seconds <= duration + 1, let player else { throw ClientError.invalidInput("The playback position is unavailable.") }
        let attempt = generation
        let position = timeline?.presentationTime(seconds) ?? seconds
        let intent = nativeIntent
        intent.seeking.withLock { $0 = true }
        defer { intent.seeking.withLock { $0 = false } }
        let success = await player.seek(to: CMTime(seconds: position, preferredTimescale: 600), toleranceBefore: .zero, toleranceAfter: .zero)
        try check(attempt)
        guard self.player === player else { throw CancellationError() }
        if !success {
            if let error = player.currentItem?.error { throw error }
            // A native AVKit scrub can supersede the restoring seek.
            if recoveringNetwork || loading {
                let current = player.currentTime().seconds
                if current.isFinite, current >= 0 { nativeRecoveryPosition = timeline?.sourceTime(current) ?? current; return }
            }
            throw ClientError.invalidInput("Could not skip to that position. Try again.")
        }
        self.seconds = seconds
        if duration <= 0 || seconds < duration { completed = false }
        saveProgress(watched: false)
    }

    func togglePlayback() { isPlaying || buffering ? pause() : resume() }
    func pause() { wantsPlayback = false; nativeIntent.playing.withLock { $0 = false }; player?.pause(); isPlaying = false; buffering = false; saveProgress(watched: false) }
    func resume() { wantsPlayback = true; nativeIntent.playing.withLock { $0 = true }; player?.playImmediately(atRate: Float(preferences.rate)); isPlaying = true }
    func checkpoint() { saveProgress(watched: false) }

    func applyPreferences(_ preferences: PlaybackPreferences) async throws {
        let valid = try PlaybackPreferences(preferences.json)
        subtitleGeneration = UUID(); subtitleDocument = nil; externalCaptions = false; presentation.showCaptions("")
        self.preferences = valid
        playbackRate = valid.rate
        selectedExternalSubtitleID = nil
        player?.defaultRate = Float(valid.rate)
        if isPlaying { player?.rate = Float(valid.rate) }
        guard let item = player?.currentItem else { return }
        selectPreferred(in: audioGroup, options: audioOptions, language: valid.audioLanguage, label: valid.audioTrack, item: item)
        if valid.subtitleLanguage == "off", let subtitleGroup { item.select(nil, in: subtitleGroup) }
        else { selectPreferred(in: subtitleGroup, options: subtitleOptions, language: valid.subtitleLanguage, label: valid.subtitleTrack, item: item) }
        let external = source?.subtitles ?? []
        if valid.subtitleLanguage == "off" { subtitleDocument = nil; externalCaptions = false; presentation.showCaptions("") }
        else if let index = external.firstIndex(where: {
            !valid.subtitleTrack.isEmpty ? $0.label == valid.subtitleTrack : valid.subtitleLanguage == "auto" ? $0.isDefault : $0.language.lowercased() == valid.subtitleLanguage.lowercased()
        }) {
            do { try await selectSubtitleTrack(id: "external:\(index)") }
            catch is CancellationError { throw CancellationError() }
            catch { progressMessage = "The selected subtitles could not be loaded. Choose a track in Playback options." }
        }
    }

    func changeRate(_ rate: Double) async throws {
        guard rate.isFinite, (0.5...3).contains(rate) else { throw ClientError.invalidInput("Choose a playback speed between 0.5× and 3×.") }
        guard let client, let item = currentItem, let player else { throw ClientError.unavailable }
        var next = preferences
        next.rate = rate
        let attempt = generation
        // Session controls work offline and preserve the selected audio/captions.
        preferences = next
        playbackRate = rate
        player.defaultRate = Float(rate)
        if isPlaying { player.rate = Float(rate) }
        // Serialize writes so an older response cannot overwrite a newer choice.
        let previous = savingRate
        savingRate = Task { [weak self] in
            await previous?.value
            guard let self, !Task.isCancelled, self.generation == attempt, self.preferences.rate == rate else { return }
            do {
                let saved = try await client.savePlaybackPreferences(itemID: item.id, preferences: next)
                try self.check(attempt)
                guard saved.playback.rate == rate else { throw ClientError.invalidResponse }
            } catch is CancellationError {} catch {
                if self.generation == attempt, self.preferences.rate == rate {
                    self.progressMessage = "Speed changed on this device. Your Server preference couldn’t be saved."
                }
            }
        }
    }

    private func loadTracks(_ item: AVPlayerItem, attempt: UUID) async throws {
        let audio = Task { @MainActor in
            let group = try await item.asset.loadMediaSelectionGroup(for: .audible)
            try check(attempt)
            audioGroup = group
        }
        defer { audio.cancel() }
        try await withTaskCancellationHandler {
            let subtitles = try await item.asset.loadMediaSelectionGroup(for: .legible)
            try await audio.value
            try check(attempt)
            subtitleGroup = subtitles
        } onCancel: { audio.cancel() }
        try check(attempt)
        audioOptions = audioGroup?.options ?? []
        subtitleOptions = subtitleGroup?.options ?? []
        audioTracks = audioOptions.enumerated().map { PlayerTrack(id: String($0.offset), title: $0.element.displayName) }
        subtitleTracks = subtitleOptions.enumerated().map { PlayerTrack(id: String($0.offset), title: $0.element.displayName) }
        subtitleTracks += (source?.subtitles ?? []).enumerated().map { PlayerTrack(id: "external:\($0.offset)", title: $0.element.label) }
    }

    private func selectPreferred(in group: AVMediaSelectionGroup?, options: [AVMediaSelectionOption], language: String, label: String, item: AVPlayerItem) {
        guard let group else { return }
        if let option = options.first(where: { !label.isEmpty && $0.displayName == label }) { item.select(option, in: group) }
        else if language != "auto", let option = options.first(where: { $0.extendedLanguageTag?.lowercased() == language.lowercased() || $0.locale?.language.languageCode?.identifier == language }) { item.select(option, in: group) }
        else { item.selectMediaOptionAutomatically(in: group) }
    }

    func selectAudioTrack(id: String) throws {
        guard let group = audioGroup, let index = Int(id), String(index) == id, audioOptions.indices.contains(index) else { throw ClientError.invalidInput("The audio track is unavailable.") }
        player?.currentItem?.select(audioOptions[index], in: group)
    }

    func selectSubtitleTrack(id: String?) async throws {
        let selection = UUID(); subtitleGeneration = selection
        if let id, id.hasPrefix("external:") {
            guard let index = Int(id.dropFirst(9)), id == "external:\(index)", let subtitles = source?.subtitles, subtitles.indices.contains(index), let client else { throw ClientError.invalidInput("The subtitle track is unavailable.") }
            let attempt = generation
            let (data, type) = try await client.resource(subtitles[index].url.absoluteString, maximum: 2 * 1024 * 1024)
            guard type == "text/vtt" || type == "text/plain" else { throw ClientError.invalidResponse }
            let document = try SubtitleDocument(data: data)
            try check(attempt)
            guard subtitleGeneration == selection else { throw CancellationError() }
            if let group = subtitleGroup { player?.currentItem?.select(nil, in: group) }
            subtitleDocument = document; externalCaptions = true
            selectedExternalSubtitleID = id
            presentation.showCaptions(document.text(at: seconds))
            return
        }
        if id == nil {
            if let group = subtitleGroup { player?.currentItem?.select(nil, in: group) }
            subtitleDocument = nil; externalCaptions = false; presentation.showCaptions("")
            selectedExternalSubtitleID = nil
            return
        }
        guard let group = subtitleGroup else { throw ClientError.invalidInput("This stream has no selectable subtitles.") }
        if let id {
            guard let index = Int(id), String(index) == id, subtitleOptions.indices.contains(index) else { throw ClientError.invalidInput("The subtitle track is unavailable.") }
            player?.currentItem?.select(subtitleOptions[index], in: group)
            subtitleDocument = nil; externalCaptions = false; presentation.showCaptions("")
            selectedExternalSubtitleID = nil
        }
    }

    func setSleepTimer(deadline: Date?) throws {
        if let deadline { guard deadline.timeIntervalSinceNow > 0, deadline.timeIntervalSinceNow <= 12 * 3600 else { throw ClientError.invalidInput("Choose a sleep timer within 12 hours.") } }
        sleepDeadline = deadline
    }

    func playQueue(_ items: [MediaItem], at index: Int, client: ServerClient, store: ProgressSyncStore) async throws {
        try queue.replace(items, startingAt: index)
        try await play(items[index], client: client, store: store)
    }

    func nextTrack() async throws {
        guard let next = queue.next(), let client, let store else { return }
        try await play(next, client: client, store: store)
    }

    func previousTrack() async throws {
        if seconds > 3 { try await seek(to: 0); return }
        guard let previous = queue.previous(), let client, let store else { return }
        try await play(previous, client: client, store: store)
    }

    private func didEnd() async {
        completed = true
        if let currentItem { onCompleted?(currentItem, source?.nextItemID) }
        saveProgress(watched: true)
        isPlaying = false
        guard let next = queue.next(automatic: true), let client, let store else { return }
        do { try await play(next, client: client, store: store) }
        catch { message = Self.playbackMessage(error) }
    }

    private func saveProgress(watched: Bool) {
        guard let writer else { return }
        let position = seconds
        let watched = watched || completed
        lastSaved = position
        let attempt = generation
        writing = Task { [weak self] in
            do {
                let synced = try await writer.update(seconds: position, watched: watched)
                guard let self, self.generation == attempt else { return }
                self.progressMessage = synced ? nil : "Progress is saved on this device. Open Progress sync to retry or resolve a conflict."
            } catch { if let self, self.generation == attempt { self.progressMessage = "Couldn’t save your position. \(AppSession.message(error))" } }
        }
    }

    private func observeAudioSession() {
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

    func stop(clearQueue: Bool = true) {
        saveProgress(watched: false)
        generation = UUID()
        wantsPlayback = false; recoveringNetwork = false; networkRecoveries = 0; networkStableSince = nil; recoveryPosition = nil
        monitoring?.cancel(); monitoring = nil
        savingRate?.cancel(); savingRate = nil
        player?.pause()
        removeTimeObserver()
        nativeIntent = NativePlaybackIntent(); nativeRecoveryPosition = nil
        notifications.forEach(NotificationCenter.default.removeObserver)
        notifications.removeAll()
        if let endObserver { NotificationCenter.default.removeObserver(endObserver); self.endObserver = nil }
        nowPlaying.deactivate()
        presentation.clear()
        player = nil; currentItem = nil; source = nil; writer = nil
        audioOptions = []; subtitleOptions = []; audioTracks = []; subtitleTracks = []
        audioGroup = nil; subtitleGroup = nil
        subtitleDocument = nil; subtitleGeneration = UUID(); externalCaptions = false; lastNowPlayingSecond = -1
        selectedExternalSubtitleID = nil; playbackRate = 1
        loading = false; isPlaying = false; buffering = false; message = nil; progressMessage = nil
        seconds = 0; duration = 0; lastSaved = -30; sleepDeadline = nil; completed = false; timeline = nil; usingCompatibility = false
        if clearQueue { queue.clear() }
        let previous = transport
        transport = MediaTransport()
        Task { await previous.close() }
    }

    private func removeTimeObserver() {
        rateObservation?.invalidate(); rateObservation = nil
        if let jumpObserver { NotificationCenter.default.removeObserver(jumpObserver); self.jumpObserver = nil }
        if let timeObserver { player?.removeTimeObserver(timeObserver); self.timeObserver = nil }
    }
    private func check(_ attempt: UUID) throws { try Task.checkCancellation(); guard attempt == generation else { throw CancellationError() } }

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

    private static func playbackMessage(_ error: Error?) -> String {
        if isNetworkFailure(error) { return "The connection is still unavailable. Your position is saved. Reopen this title when the connection returns." }
        if let error = error as? ClientError { return error.localizedDescription }
        return "This media could not be played. Check the Server or try a different title."
    }
}
