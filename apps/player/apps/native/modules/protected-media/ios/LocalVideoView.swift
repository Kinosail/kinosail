import ExpoModulesCore
import UIKit
import AVKit
import VLCKit

final class LocalVideoView: ExpoView, VLCMediaPlayerDelegate, VLCPictureInPictureDrawable, VLCPictureInPictureMediaControlling {
  let onPictureInPictureReady = EventDispatcher()
  let onPaused = EventDispatcher()
  let onFirstFrame = EventDispatcher()
  let onBuffering = EventDispatcher()
  var sourceURI = ""
  var playbackRate: Double = 1
  var nightMode = false
  var dialogueBoost = false
  var volumeBoost: Double = 1
  var audioOnly = false
  var sleepDeadline: Double = 0 { didSet { scheduleSleep() } }
  var sleepPosition: Double = 0
  private var sleepTimer: Timer?
  private func scheduleSleep() {
    sleepTimer?.invalidate(); sleepTimer = nil
    guard sleepDeadline.isFinite, sleepDeadline > 0 else { return }
    let delay = sleepDeadline / 1000 - Date().timeIntervalSince1970
    guard delay > 0, delay <= 86400 else { if delay <= 0 { pause() }; return }
    sleepTimer = Timer.scheduledTimer(withTimeInterval: delay, repeats: false) { [weak self] _ in self?.pause() }
  }
  private var audioProcessor: KinosailAudioProcessor?
  var startSeconds: Double = 0
  private var frameTimer: Timer?
  private var firstFrameReported = false
  private var pipStarting = false
  private var pip: VLCPictureInPictureWindowControlling?
  private var pipActive = false
  let onPlaying = EventDispatcher()
  let onProgress = EventDispatcher()
  let onEnd = EventDispatcher()
  let onError = EventDispatcher()
  let onTracks = EventDispatcher()
  private let player = VLCMediaPlayer(options: ["--quiet", "--no-video-title-show"])
  private var loadedURI = ""
  private var lastTime: Double = 0
  private var requestedPause = false
  private var active = false
  private var backgroundObserver: NSObjectProtocol?

  required init(appContext: AppContext?) {
    super.init(appContext: appContext)
    clipsToBounds = true
    backgroundColor = .black
    player.drawable = self
    try? AVAudioSession.sharedInstance().setCategory(.playback, mode: .moviePlayback)
    try? AVAudioSession.sharedInstance().setActive(true)
    player.delegate = self
    player.timeChangeUpdateInterval = 1
    backgroundObserver = NotificationCenter.default.addObserver(forName: UIApplication.didEnterBackgroundNotification, object: nil, queue: .main) { [weak self] _ in if self?.audioOnly != true && self?.pipActive != true && self?.pipStarting != true { self?.player.pause() } }
  }
  override func didMoveToWindow() {
    super.didMoveToWindow()
    if window == nil && !pipActive && !pipStarting { player.pause() }
  }
  func load(_ uri: String) {
    guard uri != loadedURI else { return }
    guard loadedURI.isEmpty else { onError([:]); return }
    guard startSeconds.isFinite, (0...31_536_000).contains(startSeconds) else { onError([:]); return }
    guard uri.utf8.count <= 2048, let url = URL(string: uri) else { onError([:]); return }
    let loopback = url.scheme == "http" && url.host == "127.0.0.1" && url.port != nil &&
      url.user == nil && url.password == nil && url.query == nil && url.fragment == nil &&
      url.path.range(of: "^/[A-Fa-f0-9-]{72}$", options: .regularExpression) != nil
    guard loopback || approvedOfflineURL(uri) != nil else { onError([:]); return }
    guard playbackRate.isFinite, (0.5...3).contains(playbackRate), volumeBoost.isFinite, (1...2).contains(volumeBoost) else { onError([:]); return }
    loadedURI = uri
    player.stop()
    if nightMode || dialogueBoost || volumeBoost > 1 {
      audioProcessor = KinosailAudioProcessor(player: player, nightMode: nightMode, dialogueBoost: dialogueBoost, volumeBoost: volumeBoost)
      guard audioProcessor != nil else { onError([:]); return }
      audioProcessor?.onFailure = { [weak self] in self?.onError([:]) }
    }
    player.rate = Float(playbackRate)
    guard let media = VLCMedia(url: url) else { onError([:]); return }
    media.addOption(":network-caching=500")
    media.addOption(":start-time=\(startSeconds)")
    player.media = media
    lastTime = startSeconds
    firstFrameReported = false
    frameTimer?.invalidate(); frameTimer = nil
    if !requestedPause { play() }
  }
  private func observeFirstFrame() {
    guard !firstFrameReported, frameTimer == nil else { return }
    let deadline = Date().addingTimeInterval(30)
    frameTimer = Timer.scheduledTimer(withTimeInterval: 0.05, repeats: true) { [weak self] timer in
      guard let self else { timer.invalidate(); return }
      if (self.player.media?.statistics.displayedPictures ?? 0) > 0 {
        timer.invalidate(); self.frameTimer = nil; self.firstFrameReported = true; self.onFirstFrame([:])
      } else if Date() > deadline { timer.invalidate(); self.frameTimer = nil }
    }
  }
  func setPaused(_ paused: Bool) {
    requestedPause = paused
    if paused { pause() } else if !loadedURI.isEmpty { play() }
  }
  func setRate(_ rate: Double) {
    guard rate.isFinite, (0.5...3).contains(rate) else { return }
    playbackRate = rate
    player.rate = Float(rate)
  }
  func seek(_ seconds: Double) {
    guard seconds.isFinite, seconds >= 0, seconds <= 31_536_000 else { return }
    player.time = VLCTime(number: NSNumber(value: seconds * 1000))
  }
  func selectAudio(_ index: Int) {
    guard index >= 0, index < player.audioTracks.count else { return }
    player.audioTracks[index].isSelectedExclusively = true
    tracks()
  }
  func selectSubtitle(_ index: Int) {
    if index == -1 { player.deselectAllTextTracks() }
    else if index >= 0 && index < player.textTracks.count { player.textTracks[index].isSelectedExclusively = true }
    tracks()
  }
  func delay(_ amount: Int, audio: Bool) {
    guard (-30_000_000...30_000_000).contains(amount) else { return }
    if audio { player.currentAudioPlaybackDelay = amount } else { player.currentVideoSubTitleDelay = amount }
  }
  func tracks() {
    let audio = Array(player.audioTracks.prefix(128))
    let subtitle = Array(player.textTracks.prefix(128))
    onTracks([
      "audio": audio.enumerated().map { ["id": $0.offset, "name": String(decoding: $0.element.trackName.utf8.prefix(240), as: UTF8.self), "language": String(($0.element.language ?? "").prefix(32))] },
      "subtitle": subtitle.enumerated().map { ["id": $0.offset, "name": String(decoding: $0.element.trackName.utf8.prefix(240), as: UTF8.self), "language": String(($0.element.language ?? "").prefix(32))] },
      "audioIndex": audio.firstIndex(where: { $0.isSelected }) ?? -1,
      "subtitleIndex": subtitle.firstIndex(where: { $0.isSelected }) ?? -1
    ])
  }
  func mediaPlayerStateChanged(_ newState: VLCMediaPlayerState) {
    DispatchQueue.main.async { [weak self] in
      guard let self else { return }
      self.pip?.invalidatePlaybackState()
      switch newState {
      case .playing: self.active = true; self.onPlaying([:]); self.tracks()
      case .paused: self.onPaused([:])
      case .error: self.active = false; self.onError([:])
      case .stopped:
        let duration = (self.player.media?.length.value?.doubleValue ?? 0) / 1000
        if self.active && duration > 0 && self.lastTime >= duration - 2 { self.onEnd([:]) }
        self.active = false
      default: break
      }
    }
  }
  func mediaPlayerBufferingChanged(_ progress: Float) {
    guard progress.isFinite, (0...1).contains(progress) else { return }
    DispatchQueue.main.async { [weak self] in self?.onBuffering(["active": progress < 1]) }
  }
  func mediaPlayerTimeChanged(_ notification: Notification) {
    DispatchQueue.main.async { [weak self] in
      guard let self else { return }
      let seconds = (self.player.time.value?.doubleValue ?? 0) / 1000
      if self.sleepPosition.isFinite && self.sleepPosition > 0 && seconds >= self.sleepPosition { self.sleepPosition = 0; self.pause() }
      if seconds.isFinite && seconds >= 0 { self.lastTime = seconds; self.onProgress(["currentTime": seconds]) }
    }
  }
  func mediaController() -> VLCPictureInPictureMediaControlling { self }
  func pictureInPictureReady() -> ((VLCPictureInPictureWindowControlling?) -> Void)! {
    { [weak self] controller in
      guard let controller else { return }
      DispatchQueue.main.async {
        guard let self else { return }
        self.pip = controller
        controller.stateChangeEventHandler = { [weak self] started in
          DispatchQueue.main.async {
            self?.pipActive = started
            self?.pipStarting = false
            if !started && UIApplication.shared.applicationState == .background { self?.player.pause() }
          }
        }
        self.onPictureInPictureReady([:])
      }
    }
  }
  func startPictureInPicture() {
    guard let pip else { return }
    pipStarting = true
    pip.startPictureInPicture()
    DispatchQueue.main.asyncAfter(deadline: .now() + 5) { [weak self] in
      guard let self else { return }
      self.pipStarting = false
      if !self.pipActive && UIApplication.shared.applicationState == .background { self.player.pause() }
    }
  }
  func play() { player.play(); observeFirstFrame() }
  func pause() { frameTimer?.invalidate(); frameTimer = nil; player.pause() }
  func seek(by offset: Int64, completion: @escaping () -> Void) {
    let milliseconds = Int32(clamping: offset)
    if !player.jump(withOffset: milliseconds, completion: completion) { completion() }
  }
  func mediaLength() -> Int64 { player.media?.length.value?.int64Value ?? 0 }
  func mediaTime() -> Int64 { player.time.value?.int64Value ?? 0 }
  func isMediaSeekable() -> Bool { player.isSeekable }
  func isMediaPlaying() -> Bool { player.isPlaying }
  deinit {
    sleepTimer?.invalidate()
    frameTimer?.invalidate()
    if let backgroundObserver { NotificationCenter.default.removeObserver(backgroundObserver) }
    pip?.stopPictureInPicture(); pip?.stateChangeEventHandler = { _ in };
    audioProcessor?.close()
    player.delegate = nil; player.stop(); player.drawable = nil
  }
}
