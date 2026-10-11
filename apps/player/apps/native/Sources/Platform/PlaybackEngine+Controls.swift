import AVFoundation
import AVKit
import Foundation

extension PlaybackEngine {
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

    func seek(to seconds: Double) async throws {
        try Input.position(seconds)
        guard duration == 0 || seconds <= duration + 1 else { throw ClientError.invalidInput("The playback position is unavailable.") }
        if recoveringNetwork || loading { recoveryPosition = seconds; self.seconds = seconds; return }
        try await seekPlayer(to: seconds)
    }

    func seekPlayer(to seconds: Double) async throws {
        try Input.position(seconds)
        guard duration == 0 || seconds <= duration + 1, let player else { throw ClientError.invalidInput("The playback position is unavailable.") }
        let attempt = generation
        let position = timeline?.presentationTime(seconds) ?? seconds
        let success = await player.seek(to: CMTime(seconds: position, preferredTimescale: 600), toleranceBefore: .zero, toleranceAfter: .zero)
        try check(attempt)
        guard self.player === player else { throw CancellationError() }
        if !success {
            if let error = player.currentItem?.error { throw error }
            // Retry the changed explicit target if a user superseded this seek.
            // Other interruptions must not silently replace saved progress.
            if recoveringNetwork || loading, let requested = recoveryPosition ?? nativeRecoveryPosition,
               abs(requested - seconds) >= 0.01 {
                return
            }
            throw ClientError.invalidInput("Could not skip to that position. Try again.")
        }
        self.seconds = seconds
        if duration <= 0 || seconds < duration { completed = false }
        saveProgress(watched: false)
    }

    func togglePlayback() { isPlaying || buffering ? pause() : resume() }
    func pause() { wantsPlayback = false; nativeIntent.playing.withLock { $0 = false }; player?.pause(); isPlaying = false; buffering = false; saveProgress(watched: false) }
    func resume() {
        wantsPlayback = true; nativeIntent.playing.withLock { $0 = true }
        player?.play()
        isPlaying = player?.timeControlStatus == .playing
        buffering = player?.timeControlStatus == .waitingToPlayAtSpecifiedRate || (player == nil && (loading || recoveringNetwork))
    }
    func checkpoint() { saveProgress(watched: false) }

    func applyPreferences(_ preferences: PlaybackPreferences) async throws {
        let valid = try PlaybackPreferences(preferences.json)
        let effectsChanged = valid.nightMode != self.preferences.nightMode || valid.dialogueBoost != self.preferences.dialogueBoost
        if effectsChanged, source != nil, player != nil, let client, let currentItem {
            let attempt = generation
            let details = try await client.playback(itemID: currentItem.id)
            try check(attempt)
            if valid.audioEnhancementsEnabled, details.direct != nil || details.compatible == nil {
                throw ClientError.invalidInput("Update Kinosail Server to use audio enhancements.")
            }
            let position = seconds
            let shouldResume = nativeIntent.playing.withLock { $0 } ?? wantsPlayback
            self.preferences = valid
            source = details; subtitlePolicy = details.subtitlePolicy
            #if os(tvOS)
            presentation.limitSubtitleLanguages(to: details.subtitlePickerLimited ? details.subtitleLanguage : nil)
            #endif
            playbackPreparation = nil
            playbackPreparationRequest?.cancel(); playbackPreparationRequest = nil
            try await install(details: details, compatible: valid.audioEnhancementsEnabled || details.direct == nil, at: position, attempt: attempt)
            try check(attempt)
            guard !completed else { return }
            beginMonitoring(attempt: attempt)
            if shouldResume { player?.play() }
            return
        }
        subtitleGeneration = UUID(); subtitleDocument = nil; externalCaptions = false; presentation.showCaptions("")
        self.preferences = valid
        if effectsChanged, source == nil, valid.audioEnhancementsEnabled {
            progressMessage = "Audio enhancements need a Server stream and aren’t applied to this download."
        }
        playbackRate = valid.rate
        selectedExternalSubtitleID = nil
        player?.defaultRate = Float(valid.rate)
        if player?.timeControlStatus != .paused { player?.rate = Float(valid.rate) }
        guard let item = player?.currentItem else { return }
        selectPreferred(in: audioGroup, options: audioOptions, language: valid.audioLanguage, label: valid.audioTrack, item: item)
        if (valid.subtitleLanguage == "off" || subtitlePolicy == nil), let subtitleGroup { item.select(nil, in: subtitleGroup) }
        else if subtitlePolicy?.limited == true, let subtitleGroup {
            item.select(subtitleOptions.first(where: { allowsSubtitleLanguage($0.extendedLanguageTag ?? $0.locale?.identifier ?? "") }), in: subtitleGroup)
        } else { selectPreferred(in: subtitleGroup, options: subtitleOptions, language: valid.subtitleLanguage, label: valid.subtitleTrack, item: item) }
        let external = source?.subtitles ?? []
        let rememberedOn = devicePreferencesScope.flatMap { DevicePlaybackChoices.load(scope: $0).subtitleLanguage }.map { $0 != "off" } ?? false
        let preferredExternal = subtitlePolicy?.limited == true
            ? external.firstIndex(where: { $0.isDefault }) ?? (valid.subtitleLanguage != "auto" && valid.subtitleLanguage != "off" ? external.indices.first : nil)
            : external.firstIndex(where: { !valid.subtitleTrack.isEmpty && $0.label == valid.subtitleTrack })
            ?? external.firstIndex(where: { valid.subtitleLanguage == "auto" ? $0.isDefault : $0.language.lowercased() == valid.subtitleLanguage.lowercased() })
            ?? (rememberedOn && selectedSubtitleTrackID == nil && subtitleOptions.isEmpty ? external.indices.first : nil)
        if valid.subtitleLanguage == "off" { subtitleDocument = nil; externalCaptions = false; presentation.showCaptions("") }
        else if let index = preferredExternal {
            do { try await selectSubtitleTrack(id: "external:\(index)", remember: false) }
            catch is CancellationError { throw CancellationError() }
            catch { progressMessage = "The selected subtitles could not be loaded. Choose a track in Playback options." }
        }
        if rememberedOn, selectedSubtitleTrackID == nil, let subtitleGroup,
           let first = subtitleOptions.first(where: { allowsSubtitleLanguage($0.extendedLanguageTag ?? $0.locale?.identifier ?? "") }) {
            item.select(first, in: subtitleGroup)
        }
        observedAudioTrackID = selectedAudioTrackID
        observedSubtitleTrackID = selectedSubtitleTrackID
    }

    func changeRate(_ rate: Double) async throws {
        guard rate.isFinite, (0.5...3).contains(rate) else { throw ClientError.invalidInput("Choose a playback speed between 0.5× and 3×.") }
        guard currentItem != nil, let player, devicePreferencesScope != nil else { throw ClientError.unavailable }
        preferences.rate = rate
        playbackRate = rate
        player.defaultRate = Float(rate)
        if player.timeControlStatus != .paused { player.rate = Float(rate) }
        rememberChoices { $0.rate = rate }
    }

    func rememberChoices(_ change: (inout DevicePlaybackChoices) -> Void) {
        guard let scope = devicePreferencesScope else { return }
        var choices = DevicePlaybackChoices.load(scope: scope)
        change(&choices)
        choices.save(scope: scope)
    }

    func rememberAudioSelection(id: String) {
        guard let index = Int(id), audioOptions.indices.contains(index) else { return }
        observedAudioTrackID = id
        let option = audioOptions[index]
        rememberChoices {
            $0.audioLanguage = (try? PlaybackPreferences.language((option.extendedLanguageTag ?? option.locale?.identifier ?? "auto").lowercased(), subtitle: false)) ?? "auto"
            $0.audioTrack = (try? Input.text(option.displayName, max: 256, label: "audio track", empty: true)) ?? ""
        }
    }

    func rememberSubtitleSelection(id: String?) {
        observedSubtitleTrackID = id
        if let id, id.hasPrefix("external:"), let index = Int(id.dropFirst(9)), let tracks = source?.subtitles, tracks.indices.contains(index) {
            let track = tracks[index]
            rememberChoices {
                $0.subtitleLanguage = (try? PlaybackPreferences.language(track.language.lowercased(), subtitle: true)) ?? "auto"
                $0.subtitleTrack = (try? Input.text(track.label, max: 256, label: "subtitle track", empty: true)) ?? ""
            }
        } else if let id, let index = Int(id), subtitleOptions.indices.contains(index) {
            let option = subtitleOptions[index]
            rememberChoices {
                $0.subtitleLanguage = (try? PlaybackPreferences.language((option.extendedLanguageTag ?? option.locale?.identifier ?? "auto").lowercased(), subtitle: true)) ?? "auto"
                $0.subtitleTrack = (try? Input.text(option.displayName, max: 256, label: "subtitle track", empty: true)) ?? ""
            }
        } else if id == nil {
            rememberChoices { $0.subtitleLanguage = "off"; $0.subtitleTrack = "" }
        }
    }

    func allowsSubtitleLanguage(_ language: String) -> Bool { subtitlePolicy?.allows(language) ?? false }

    func loadTracks(_ item: AVPlayerItem, attempt: UUID) async throws {
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
        var regularShown = false
        subtitleTracks = subtitleOptions.enumerated().compactMap { index, option in
            guard allowsSubtitleLanguage(option.extendedLanguageTag ?? option.locale?.identifier ?? "") else { return nil }
            if subtitlePolicy?.limited == true && !option.hasMediaCharacteristic(.containsOnlyForcedSubtitles) {
                guard !regularShown else { return nil }
                regularShown = true
            }
            return PlayerTrack(id: String(index), title: PlayerTrack.embeddedSubtitleTitle(option.displayName))
        }
        subtitleTracks += (source?.subtitles ?? []).enumerated().map { PlayerTrack(id: "external:\($0.offset)", title: $0.element.label) }
    }

    func selectPreferred(in group: AVMediaSelectionGroup?, options: [AVMediaSelectionOption], language: String, label: String, item: AVPlayerItem) {
        guard let group else { return }
        if let option = options.first(where: { !label.isEmpty && $0.displayName == label }) { item.select(option, in: group) }
        else if language != "auto", let option = options.first(where: { $0.extendedLanguageTag?.lowercased() == language.lowercased() || $0.locale?.language.languageCode?.identifier == language }) { item.select(option, in: group) }
        else { item.selectMediaOptionAutomatically(in: group) }
    }

    func selectAudioTrack(id: String) throws {
        guard let group = audioGroup, let index = Int(id), String(index) == id, audioOptions.indices.contains(index) else { throw ClientError.invalidInput("The audio track is unavailable.") }
        player?.currentItem?.select(audioOptions[index], in: group)
        rememberAudioSelection(id: id)
    }

    func selectSubtitleTrack(id: String?, remember: Bool = true) async throws {
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
            if remember { rememberSubtitleSelection(id: id) }
            return
        }
        if id == nil {
            if let group = subtitleGroup { player?.currentItem?.select(nil, in: group) }
            subtitleDocument = nil; externalCaptions = false; presentation.showCaptions("")
            selectedExternalSubtitleID = nil
            if remember { rememberSubtitleSelection(id: nil) }
            return
        }
        guard let group = subtitleGroup else { throw ClientError.invalidInput("This stream has no selectable subtitles.") }
        if let id {
            guard let index = Int(id), String(index) == id, subtitleOptions.indices.contains(index),
                  allowsSubtitleLanguage(subtitleOptions[index].extendedLanguageTag ?? subtitleOptions[index].locale?.identifier ?? "") else { throw ClientError.invalidInput("The subtitle track is unavailable.") }
            player?.currentItem?.select(subtitleOptions[index], in: group)
            subtitleDocument = nil; externalCaptions = false; presentation.showCaptions("")
            selectedExternalSubtitleID = nil
            if remember { rememberSubtitleSelection(id: id) }
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

    func saveProgress(watched: Bool) {
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


}
