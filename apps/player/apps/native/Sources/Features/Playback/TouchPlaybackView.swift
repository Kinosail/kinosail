#if os(iOS)
import AVKit
import SwiftUI

struct TouchPlaybackView: View {
    let failure: String?
    let retry: () -> Void
    let close: () -> Void
    @Environment(AppSession.self) private var session
    @Environment(\.accessibilityVoiceOverEnabled) private var voiceOver
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    @Environment(\.dynamicTypeSize) private var typeSize
    @Environment(\.verticalSizeClass) private var verticalSize
    @State private var controlsVisible = true
    @State private var interaction = 0
    @State private var scrubbing = false
    @State private var scrubPosition = 0.0
    @State private var previewImage: UIImage?
    @State private var previewUnavailable = false
    @State private var actionMessage: String?
    @State private var sheet: PlaybackSheet?
    @State private var switchControl = UIAccessibility.isSwitchControlRunning
    @State private var showsVolume = false
    @State private var seekRevision = 0
    @State private var pendingSeek: Double?
    @State private var timelineHeight: CGFloat = 144

    private var playback: PlaybackCoordinator { session.player }
    private var presentation: PlayerPresentation { playback.presentation }
    private var displayedPosition: Double { scrubbing ? scrubPosition : pendingSeek ?? playback.seconds }
    private var compactControls: Bool { verticalSize == .compact && typeSize.isAccessibilitySize }
    private var error: String? { failure ?? (playback.recoveringNetwork ? nil : playback.message) }
    private var pending: String? {
        if playback.recoveringNetwork { return "Reconnecting…" }
        if playback.loading || playback.player == nil { return "Opening video…" }
        if playback.buffering { return "Buffering…" }
        if !presentation.readyForDisplay && !presentation.pictureInPicture { return "Preparing video…" }
        return nil
    }
    private var canHide: Bool {
        playback.isPlaying && pending == nil && error == nil && !presentation.pictureInPicture && !scrubbing && sheet == nil && !showsVolume && !voiceOver && !switchControl
    }

    var body: some View {
        ZStack {
            TouchVideoView(player: playback.player, presentation: presentation, controlsVisible: controlsVisible, controlsInset: timelineHeight + 16,
                           restore: { session.showsVideoPlayer = true })
                .ignoresSafeArea()
            GeometryReader { geometry in
                Color.clear.contentShape(Rectangle())
                    .gesture(SpatialTapGesture(count: 2).onEnded { tap in
                        guard playback.player != nil, playback.duration > 0, !presentation.pictureInPicture else { return }
                        seek(displayedPosition + (tap.location.x < geometry.size.width / 2 ? -10 : 10))
                    }.exclusively(before: TapGesture().onEnded { reveal(toggle: true) }))
                    .accessibilityLabel("Show playback controls")
                    .accessibilityAddTraits(.isButton)
                    .accessibilityAction { reveal() }
                    .accessibilityHidden(controlsVisible || presentation.pictureInPicture).allowsHitTesting(!presentation.pictureInPicture)
            }
            if presentation.pictureInPicture {
                TouchPictureInPictureView { presentation.stopPictureInPicture() }
            }
            GeometryReader { geometry in
                let area = TouchPlaybackLayout.controlArea(size: geometry.size, division: geometry.playbackDivision)
                VStack(spacing: 0) {
                    header
                    Spacer(minLength: 12)
                    if let error {
                        ContentUnavailableView {
                            Label("Playback interrupted", systemImage: "exclamationmark.triangle")
                        } description: { Text(error) } actions: {
                            Button("Try again") { actionMessage = nil; retry(); reveal() }
                                .buttonStyle(.borderedProminent).foregroundStyle(.black)
                        }
                    } else {
                        VStack(spacing: 20) {
                            if let pending { Text(pending).font(.subheadline).foregroundStyle(.white) }
                            if !compactControls { transport }
                        }
                    }
                    Spacer(minLength: 12)
                    if error == nil { timeline }
                }
                .frame(width: area.width, height: area.height).position(x: area.midX, y: area.midY)
            }
            .opacity(!presentation.pictureInPicture && (controlsVisible || error != nil) ? 1 : 0).allowsHitTesting(!presentation.pictureInPicture && (controlsVisible || error != nil))
            .accessibilityHidden(presentation.pictureInPicture || (!controlsVisible && error == nil))
        }
        .background(.black)
        .foregroundStyle(.white)
        .tint(.white)
        .preferredColorScheme(.dark)
        .statusBarHidden(true)
        .sheet(item: $sheet, onDismiss: { presentation.showingOptions = false; reveal() }) { selection in
            NavigationStack {
                switch selection {
                case .options: PlaybackToolsScreen()
                case .tracks: PlaybackTracksScreen()
                case .speed: PlaybackSpeedScreen()
                }
            }
            .presentationDetents([.medium, .large])
            .presentationDragIndicator(.visible)
        }
        .task(id: interaction) {
            guard canHide else { return }
            do { try await Task.sleep(for: .seconds(4)) } catch { return }
            guard canHide else { return }
            withAnimation(reduceMotion ? nil : .easeOut(duration: 0.2)) { controlsVisible = false }
        }
        .onChange(of: canHide) { _, _ in reveal() }
        .onChange(of: sheet) { _, selection in presentation.showingOptions = selection != nil }
        .onChange(of: playback.currentItem?.id) { _, _ in
            actionMessage = nil; scrubbing = false; pendingSeek = nil; previewImage = nil; seekRevision += 1; reveal()
        }
        .onReceive(NotificationCenter.default.publisher(for: UIAccessibility.switchControlStatusDidChangeNotification)) { _ in
            switchControl = UIAccessibility.isSwitchControlRunning
            reveal()
        }
    }

    private var header: some View {
        HStack(spacing: 12) {
            Button("Close player", systemImage: "chevron.down", action: close)
                .labelStyle(.iconOnly).frame(width: 44, height: 44)
                .keyboardShortcut(.cancelAction)
            Text(playback.currentItem?.title ?? "Playback")
                .font(.headline).lineLimit(compactControls ? 1 : 2).frame(maxWidth: .infinity, alignment: .leading)
                .accessibilityAddTraits(.isHeader)
            Button("Playback options", systemImage: "ellipsis") { sheet = .options; reveal() }
                .labelStyle(.iconOnly).frame(width: 44, height: 44)
        }
        .buttonStyle(.plain)
        .padding(.horizontal, 16).padding(.vertical, 8)
        .background(LinearGradient(colors: [.black.opacity(0.8), .clear], startPoint: .top, endPoint: .bottom).ignoresSafeArea(edges: .top))
    }

    private var transport: some View {
        HStack(spacing: 36) {
            Button("Back 10 seconds", systemImage: "gobackward.10") { seek(displayedPosition - 10) }
                .font(.title2).frame(width: 56, height: 56)
                .keyboardShortcut(.leftArrow, modifiers: [])
            Button {
                reveal()
                if playback.completed { seek(0, resume: true) }
                else { playback.togglePlayback() }
            } label: {
                Image(systemName: playback.completed ? "arrow.counterclockwise" : playback.isPlaying || playback.buffering ? "pause.fill" : "play.fill")
                    .font(.largeTitle).frame(width: 72, height: 72)
            }
            .accessibilityLabel(playback.completed ? "Replay" : playback.isPlaying || playback.buffering ? "Pause" : "Play")
            .keyboardShortcut(.space, modifiers: [])
            Button("Forward 10 seconds", systemImage: "goforward.10") { seek(displayedPosition + 10) }
                .font(.title2).frame(width: 56, height: 56)
                .keyboardShortcut(.rightArrow, modifiers: [])
        }
        .labelStyle(.iconOnly).buttonStyle(.plain)
        .background(.black.opacity(0.35), in: Capsule())
        .disabled(playback.player == nil || presentation.pictureInPicture)
    }

    private var timeline: some View {
        VStack(spacing: 2) {
            if let message = actionMessage ?? presentation.presentationMessage {
                Text(message).font(.footnote).multilineTextAlignment(.center).padding(.bottom, 8)
            }
            if scrubbing {
                VStack(spacing: 4) {
                    if let previewImage {
                        Image(uiImage: previewImage).resizable().aspectRatio(contentMode: .fit)
                            .frame(width: 160, height: 90).background(.black)
                            .clipShape(RoundedRectangle(cornerRadius: 8))
                    } else if previewUnavailable {
                        Text("Preview unavailable").font(.caption).frame(width: 160, height: 90)
                    } else {
                        RoundedRectangle(cornerRadius: 8).fill(.white.opacity(0.12))
                            .frame(width: 160, height: 90).skeletonShimmer()
                    }
                    Text(scrubPosition.clock).font(.caption.monospacedDigit())
                }
                .frame(maxWidth: .infinity)
                .accessibilityElement(children: .ignore)
                .accessibilityLabel("Preview at \(scrubPosition.clock)")
                .task(id: "\(playback.currentItem?.id ?? ""):\(Int(min(43210, max(0, scrubPosition)) / 10))") {
                    await loadPreview()
                }
            }
            Slider(value: Binding(get: { min(max(1, playback.duration), max(0, displayedPosition)) }, set: {
                if scrubbing { scrubPosition = $0 } else { seek($0) }
            }),
                   in: 0...max(1, playback.duration)) { editing in
                if editing { scrubPosition = displayedPosition; scrubbing = true; reveal() }
                else { seek(scrubPosition) }
            }
            .tint(KinoTheme.signal)
            .disabled(playback.duration <= 0 || playback.player == nil)
            .accessibilityLabel("Playback position")
            .accessibilityValue("\(displayedPosition.clock) of \(playback.duration > 0 ? playback.duration.clock : "unknown duration")")
            HStack {
                Text(displayedPosition.clock)
                Spacer()
                Text(playback.duration > 0 ? "−\(max(0, playback.duration - displayedPosition).clock)" : "Duration unavailable")
            }
            .font(.caption.monospacedDigit()).foregroundStyle(.white.opacity(0.85))
            if compactControls {
                HStack(spacing: 20) {
                    transport
                    actions(showLabel: false)
                }
            } else {
                ViewThatFits(in: .horizontal) {
                    actions(showLabel: true)
                    actions(showLabel: false)
                }
            }
        }
        .padding(.horizontal, 24).padding(.top, 24).padding(.bottom, 8)
        .background(LinearGradient(colors: [.clear, .black.opacity(0.85)], startPoint: .top, endPoint: .bottom).ignoresSafeArea(edges: .bottom))
        .onGeometryChange(for: CGFloat.self) { $0.size.height } action: { timelineHeight = $0 }
    }

    private func actions(showLabel: Bool) -> some View {
        HStack(spacing: 8) {
            Button { sheet = .tracks; reveal() } label: {
                HStack(spacing: 6) {
                    Image(systemName: playback.selectedSubtitleTrackID != nil ? "captions.bubble.fill" : "captions.bubble")
                    if showLabel { Text(playback.trackControlsTitle).fixedSize() }
                }
                .font(.subheadline)
            }
            .accessibilityLabel(playback.trackControlsTitle)
            .frame(minWidth: 44, minHeight: 44).disabled(playback.audioTracks.isEmpty && playback.subtitleTracks.isEmpty)
            Spacer(minLength: 0)
            Button("\(playback.playbackRate.formatted())×") { sheet = .speed; reveal() }
                .font(.subheadline.monospacedDigit()).frame(minWidth: 44, minHeight: 44)
                .accessibilityLabel("Playback speed, \(playback.playbackRate.formatted()) times")
            Button("Volume", systemImage: "speaker.wave.2") { showsVolume = true; reveal() }
                .labelStyle(.iconOnly).frame(width: 44, height: 44)
                .popover(isPresented: $showsVolume) {
                    SystemPlaybackVolume().frame(width: 220, height: 44).padding()
                        .presentationCompactAdaptation(.popover)
                }
            if AVPictureInPictureController.isPictureInPictureSupported() {
                Button { presentation.startPictureInPicture(); reveal() } label: {
                    Image(uiImage: AVPictureInPictureController.pictureInPictureButtonStartImage)
                }
                .frame(width: 44, height: 44)
                .accessibilityLabel("Start Picture in Picture")
                .disabled(!presentation.pictureInPicturePossible || presentation.pictureInPicture)
            }
        }
        .buttonStyle(.plain)
    }

    private func reveal(toggle: Bool = false) {
        interaction += 1
        withAnimation(reduceMotion ? nil : .easeOut(duration: 0.15)) {
            controlsVisible = toggle && !voiceOver && !switchControl ? !controlsVisible : true
        }
    }

    private func seek(_ position: Double, resume: Bool = false) {
        reveal()
        seekRevision += 1
        let request = seekRevision
        let target = min(playback.duration > 0 ? playback.duration : max(0, position), max(0, position))
        pendingSeek = target
        Task {
            do {
                try await playback.seek(to: target)
                if resume { playback.resume() }
                if request == seekRevision { actionMessage = nil }
            } catch { if request == seekRevision { actionMessage = AppSession.message(error) } }
            guard request == seekRevision else { return }
            pendingSeek = nil
            scrubbing = false
            reveal()
        }
    }

    private func loadPreview() async {
        previewImage = nil
        previewUnavailable = false
        let second = Int(min(43210, max(0, scrubPosition)) / 10) * 10
        do {
            let image = try await PlaybackFramePreview.load(template: playback.source?.trickplay,
                                                            client: session.client, asset: playback.player?.currentItem?.asset,
                                                            second: second)
            try Task.checkCancellation()
            previewImage = image
        } catch is CancellationError {} catch { if !Task.isCancelled { previewUnavailable = true } }
    }
}

private enum PlaybackSheet: String, Identifiable {
    case options, tracks, speed
    var id: String { rawValue }
}

#endif
