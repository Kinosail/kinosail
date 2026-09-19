import AVFoundation
import AVKit
import Observation
import Synchronization

struct PlayerTrack: Identifiable {
    let id: String
    let title: String
}

final class NativePlaybackIntent: Sendable {
    let playing = Mutex<Bool?>(nil)
    let seeking = Mutex(false)
}

@MainActor @Observable
final class PlaybackCoordinator {
    private let engine = PlaybackEngine()
    var player: AVPlayer? { engine.player }
    var currentItem: MediaItem? { engine.currentItem }
    var source: PlaybackSource? { engine.source }
    var seconds: Double { engine.seconds }
    var duration: Double { engine.duration }
    var loading: Bool { engine.loading }
    var isPlaying: Bool { engine.isPlaying }
    var buffering: Bool { engine.buffering }
    var usingCompatibility: Bool { engine.usingCompatibility }
    var message: String? { engine.message }
    var progressMessage: String? { engine.progressMessage }
    var audioTracks: [PlayerTrack] { engine.audioTracks }
    var subtitleTracks: [PlayerTrack] { engine.subtitleTracks }
    var externalCaptions: Bool { engine.externalCaptions }
    var playbackRate: Double { engine.playbackRate }
    var selectedExternalSubtitleID: String? { engine.selectedExternalSubtitleID }
    var recoveringNetwork: Bool { engine.recoveringNetwork }
    var completed: Bool { engine.completed }
    var sleepDeadline: Date? { engine.sleepDeadline }
    var queue: MediaQueue { engine.queue }
    var presentation: PlayerPresentation { engine.presentation }
    var selectedAudioTrackID: String? { engine.selectedAudioTrackID }
    var selectedSubtitleTrackID: String? { engine.selectedSubtitleTrackID }
    var onCompleted: (@MainActor (MediaItem, String?) -> Void)? {
        get { engine.onCompleted }
        set { engine.onCompleted = newValue }
    }

    func play(_ item: MediaItem, client: ServerClient, store: ProgressSyncStore) async throws { try await engine.play(item, client: client, store: store) }
    #if os(iOS)
    func playOffline(_ item: MediaItem, file: URL, downloadID: String, client: ServerClient, store: ProgressSyncStore, preferences: PlaybackPreferences) async throws {
        try await engine.playOffline(item, file: file, downloadID: downloadID, client: client, store: store, preferences: preferences)
    }
    #endif
    func seek(to seconds: Double) async throws { try await engine.seek(to: seconds) }
    func togglePlayback() { engine.togglePlayback() }
    func pause() { engine.pause() }
    func resume() { engine.resume() }
    func checkpoint() { engine.checkpoint() }
    func applyPreferences(_ preferences: PlaybackPreferences) async throws { try await engine.applyPreferences(preferences) }
    func changeRate(_ rate: Double) async throws { try await engine.changeRate(rate) }
    func selectAudioTrack(id: String) throws { try engine.selectAudioTrack(id: id) }
    func selectSubtitleTrack(id: String?) async throws { try await engine.selectSubtitleTrack(id: id) }
    func setSleepTimer(deadline: Date?) throws { try engine.setSleepTimer(deadline: deadline) }
    func playQueue(_ items: [MediaItem], at index: Int, client: ServerClient, store: ProgressSyncStore) async throws { try await engine.playQueue(items, at: index, client: client, store: store) }
    func nextTrack() async throws { try await engine.nextTrack() }
    func previousTrack() async throws { try await engine.previousTrack() }
    func stop(clearQueue: Bool = true) { engine.stop(clearQueue: clearQueue) }
    static func isNetworkFailure(_ error: Error?, depth: Int = 0) -> Bool { PlaybackEngine.isNetworkFailure(error, depth: depth) }
    static func isFormatFailure(_ error: Error?, depth: Int = 0) -> Bool { PlaybackEngine.isFormatFailure(error, depth: depth) }
}

@MainActor @Observable
final class PlaybackEngine {
    var player: AVPlayer?
    var currentItem: MediaItem?
    var source: PlaybackSource?
    var seconds: Double = 0
    var duration: Double = 0
    var loading = false
    var isPlaying = false
    var buffering = false
    var usingCompatibility = false
    var message: String?
    var progressMessage: String?
    var audioTracks: [PlayerTrack] = []
    var subtitleTracks: [PlayerTrack] = []
    var externalCaptions = false
    var playbackRate: Double = 1
    var selectedExternalSubtitleID: String?
    var recoveringNetwork = false
    var completed = false
    var sleepDeadline: Date?
    let queue = MediaQueue()
    let presentation = PlayerPresentation()
    @ObservationIgnored var onCompleted: (@MainActor (MediaItem, String?) -> Void)?

    @ObservationIgnored var transport = MediaTransport()
    @ObservationIgnored var generation = UUID()
    @ObservationIgnored var client: ServerClient?
    @ObservationIgnored var store: ProgressSyncStore?
    @ObservationIgnored var writer: ProgressWriter?
    @ObservationIgnored var timeline: MediaTimeline?
    @ObservationIgnored var preferences = PlaybackPreferences()
    @ObservationIgnored var timeObserver: Any?
    @ObservationIgnored var rateObservation: NSKeyValueObservation?
    @ObservationIgnored var jumpObserver: NSObjectProtocol?
    @ObservationIgnored var monitoring: Task<Void, Never>?
    @ObservationIgnored var writing: Task<Void, Never>?
    @ObservationIgnored var savingRate: Task<Void, Never>?
    @ObservationIgnored var notifications: [NSObjectProtocol] = []
    @ObservationIgnored var nowPlaying = NowPlayingController()
    @ObservationIgnored var lastSaved: Double = -30
    @ObservationIgnored var audioOptions: [AVMediaSelectionOption] = []
    @ObservationIgnored var subtitleOptions: [AVMediaSelectionOption] = []
    @ObservationIgnored var audioGroup: AVMediaSelectionGroup?
    @ObservationIgnored var subtitleGroup: AVMediaSelectionGroup?
    @ObservationIgnored var resumeAfterInterruption = false
    @ObservationIgnored var endObserver: NSObjectProtocol?
    @ObservationIgnored var subtitleDocument: SubtitleDocument?
    @ObservationIgnored var subtitleGeneration = UUID()
    @ObservationIgnored var lastNowPlayingSecond = -1
    @ObservationIgnored var nativeIntent = NativePlaybackIntent()
    @ObservationIgnored var wantsPlayback = false
    @ObservationIgnored var networkRecoveries = 0
    @ObservationIgnored var networkStableSince: Date?
    @ObservationIgnored var recoveryPosition: Double?
    @ObservationIgnored var nativeRecoveryPosition: Double?

    fileprivate init() {
        presentation.closedPictureInPicture = { [weak self] in self?.stop() }
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

    func check(_ attempt: UUID) throws { try Task.checkCancellation(); guard attempt == generation else { throw CancellationError() } }


}
