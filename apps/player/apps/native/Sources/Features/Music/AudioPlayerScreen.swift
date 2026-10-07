import SwiftUI

private enum AudioNowPlayingGeometry {
    static var transportSpacing: CGFloat {
        #if os(tvOS)
        28
        #else
        12
        #endif
    }
    static func transportSize(_ index: Int) -> CGFloat {
        #if os(tvOS)
        index == 1 ? 112 : 128
        #else
        52
        #endif
    }
    static func width(accessibility: Bool) -> CGFloat {
        #if os(tvOS)
        accessibility ? 900 : 1400
        #else
        700
        #endif
    }
    static func layout(accessibility: Bool) -> AnyLayout {
        #if os(tvOS)
        accessibility ? AnyLayout(VStackLayout(spacing: 28))
                      : AnyLayout(HStackLayout(alignment: .top, spacing: 64))
        #else
        AnyLayout(VStackLayout(spacing: 28))
        #endif
    }
}

struct AudioPlayerScreen: View {
    let itemID: String
    var hidesMiniPlayer = true
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @State private var failure: String?
    @State private var revision = 0
    @State private var seekValue: Double = 0
    @State private var seeking = false
    #if os(tvOS)
    @State private var showsTools = false
    @State private var focusedInitially = false
    @FocusState private var playFocused: Bool
    @Namespace private var audioFocus
    #endif
    private var upcoming: [MediaItem] { Array(session.player.queue.items.dropFirst((session.player.queue.currentIndex ?? 0) + 1).prefix(20)) }
    private var nowPlayingLayout: AnyLayout { AudioNowPlayingGeometry.layout(accessibility: dynamicTypeSize.isAccessibilitySize) }
    var body: some View {
        ScrollView {
            if let failure { RetryState(message: failure) { revision += 1 } }
            else if let message = session.player.message { RetryState(message: message) { revision += 1 } }
            else if let item = session.player.currentItem, item.isAudio, session.player.player != nil {
                nowPlayingLayout {
                    Artwork(path: item.artwork, symbol: "music.note", ratio: 1).frame(maxWidth: 440).clipShape(.rect(cornerRadius: 24))
                    VStack(spacing: 28) {
                        VStack(spacing: 8) {
                            Text(item.title).font(.title.bold()).multilineTextAlignment(.center)
                            Text(item.artist.isEmpty ? item.album : item.artist).foregroundStyle(.secondary)
                        }
                        if session.player.loading { Text("Opening audio…").foregroundStyle(.secondary) }
                        if let message = session.player.message ?? session.player.progressMessage { Text(message).foregroundStyle(.secondary).multilineTextAlignment(.center) }
                        VStack(spacing: 8) {
                            #if os(iOS)
                            BufferedPlaybackSlider(value: $seekValue, duration: session.player.duration, buffered: session.player.bufferedRanges, onEditingChanged: { editing in
                                seeking = editing
                                if !editing { perform { try await session.player.seek(to: seekValue) } }
                            }).disabled(session.player.duration <= 0).accessibilityLabel("Playback position").accessibilityValue("\(seekValue.clock) of \(session.player.duration.clock)")
                            #else
                            BufferedPlaybackTrack(value: session.player.seconds, duration: session.player.duration, buffered: session.player.bufferedRanges)
                                .tint(KinoTheme.signal)
                                .accessibilityLabel("Playback position")
                                .accessibilityValue("\(session.player.seconds.clock) of \(session.player.duration.clock)")
                            #endif
                            HStack { Text(session.player.seconds.clock); Spacer(); Text(session.player.duration.clock) }.font(.caption.monospacedDigit()).foregroundStyle(.secondary)
                        }
                        HStack(spacing: AudioNowPlayingGeometry.transportSpacing) {
                            Button("Back 15 seconds", systemImage: "gobackward.15") { perform { try await session.player.seek(to: max(0, session.player.seconds - 15)) } }.labelStyle(.iconOnly).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground().controlSize(.large)
                            Button { session.player.togglePlayback() } label: {
                                Label(session.player.isPlaying || session.player.buffering ? "Pause" : "Play", systemImage: session.player.isPlaying || session.player.buffering ? "pause.fill" : "play.fill")
                                    .labelStyle(.iconOnly)
                                    #if os(tvOS)
                                    .font(.system(size: 44, weight: .semibold))
                                    .frame(width: AudioNowPlayingGeometry.transportSize(1), height: AudioNowPlayingGeometry.transportSize(1))
                                    .foregroundStyle(KinoTheme.tvOSPrimaryInk)
                                    .background(KinoTheme.tvOSPrimaryFill, in: Circle())
                                    #endif
                            }
                                .font(.largeTitle)
                                #if os(tvOS)
                                .buttonStyle(.card).buttonBorderShape(.circle)
                                #else
                                .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                                #endif
                                #if os(tvOS)
                                .tvOSDefaultPlayFocus(in: audioFocus, id: "audio.play.\(item.id)", enabled: session.player.player != nil)
                                .focused($playFocused)
                                .onAppear { if !focusedInitially { focusedInitially = true; playFocused = true } }
                                #endif
                            Button("Forward 30 seconds", systemImage: "goforward.30") { perform { try await session.player.seek(to: min(session.player.duration, session.player.seconds + 30)) } }.labelStyle(.iconOnly).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground().controlSize(.large)
                        }.font(.title2).disabled(session.player.player == nil)
                        if item.kind == .music {
                            HStack(spacing: 24) {
                                Button("Previous", systemImage: "backward.end.fill") { perform { try await session.player.previousTrack() } }.secondaryControlForeground()
                                Button("Next", systemImage: "forward.end.fill") { perform { try await session.player.nextTrack() } }.secondaryControlForeground()
                            }.labelStyle(.iconOnly).font(.title2).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).controlSize(.large).disabled(session.player.player == nil)
                        }
                        ViewThatFits(in: .horizontal) {
                            HStack(spacing: 12) { audioOptions(item) }
                            VStack(spacing: 12) { audioOptions(item) }
                        }.font(.callout)
                            #if os(tvOS)
                            .tint(KinoTheme.secondaryControlTint)
                            #endif
                        if let deadline = session.player.sleepDeadline { Text("Pauses at \(deadline.formatted(date: .omitted, time: .shortened))").font(.caption).foregroundStyle(.secondary) }
                        if !upcoming.isEmpty {
                            VStack(alignment: .leading, spacing: 14) {
                                Text("Up next").font(.title2.bold())
                                ForEach(upcoming) { next in
                                    HStack { Text(next.title); Spacer(); Text(next.artist).foregroundStyle(.secondary) }.font(.callout)
                                }
                            }.frame(maxWidth: .infinity, alignment: .leading)
                        }
                    }
                }.frame(maxWidth: AudioNowPlayingGeometry.width(accessibility: dynamicTypeSize.isAccessibilitySize)).frame(maxWidth: .infinity).padding(KinoTheme.contentPadding)
            } else { AudioLoadingState().padding(KinoTheme.contentPadding) }
        }
        #if os(tvOS)
        .cinemaBackground()
        #endif
        .navigationTitle("Now playing")
        #if os(iOS)
        .background(KinoTheme.background)
        .navigationBarTitleDisplayMode(.inline)
        .toolbar { if !hidesMiniPlayer { ToolbarItem(placement: .cancellationAction) { Button("Close") { dismiss() } } } }
        #endif
        #if os(tvOS)
        .focusScope(audioFocus)
        .onPlayPauseCommand { if session.player.player != nil { session.player.togglePlayback() } }
        .onAppear { if hidesMiniPlayer { session.showsAudioPlayer = true } }
        .onDisappear { if hidesMiniPlayer { session.showsAudioPlayer = false } }
        #endif
        .toolbar { ToolbarItem(placement: .primaryAction) {
            #if os(tvOS)
            if session.player.player != nil {
            Button { showsTools = true } label: {
                Image(systemName: "ellipsis.circle.fill").font(.title2).foregroundStyle(KinoTheme.text)
            }
                .accessibilityLabel("Playback options")
            }
            #else
            NavigationLink { PlaybackToolsScreen() } label: { Label("Playback options", systemImage: "ellipsis.circle") }
            #endif
        } }
        #if os(tvOS)
        .sheet(isPresented: $showsTools) { NavigationStack { PlaybackToolsScreen() } }
        #endif
        .task(id: "\(itemID):\(revision)") { await load() }
        .onChange(of: session.player.seconds, initial: true) { _, position in if !seeking { seekValue = position } }
        .onChange(of: session.player.currentItem?.id) { previous, current in
            if previous != nil && current == nil { dismiss() }
        }
    }
    @ViewBuilder private func audioOptions(_ item: MediaItem) -> some View {
        #if os(tvOS)
        if let chapters = session.player.source?.chapters, !chapters.isEmpty {
            Menu("Chapters", systemImage: "list.bullet") {
                ForEach(chapters) { chapter in
                    Button("\(chapter.title) · \(chapter.start.clock)") {
                        perform { try await session.player.seek(to: chapter.start) }
                    }
                }
            }
        }
        #endif
        if item.kind == .music {
            Button(session.player.queue.shuffled ? "Shuffle on" : "Shuffle off", systemImage: "shuffle") { session.player.queue.setShuffle(!session.player.queue.shuffled) }
                .tint(session.player.queue.shuffled ? KinoTheme.signal : KinoTheme.muted)
                .secondaryControlForeground()
            Menu("Repeat: \(session.player.queue.repeatMode.rawValue)", systemImage: "repeat") {
                ForEach(MediaQueue.RepeatMode.allCases, id: \.self) { mode in Button(mode.rawValue.capitalized) { session.player.queue.setRepeat(mode) } }
            }
            .accessibilityLabel("Repeat: \(session.player.queue.repeatMode.rawValue)").secondaryControlForeground()
        }
        Menu("Sleep timer", systemImage: "moon") {
            ForEach([15, 30, 45, 60, 90], id: \.self) { minutes in
                Button("\(minutes) minutes") { perform { try session.player.setSleepTimer(deadline: Date().addingTimeInterval(Double(minutes) * 60)) } }
            }
            Button("Turn off") { perform { try session.player.setSleepTimer(deadline: nil) } }
        }
        .accessibilityLabel("Sleep timer").secondaryControlForeground()
    }
    private func load() async {
        failure = nil
        if session.player.currentItem?.id == itemID, session.player.player != nil || session.player.loading, revision == 0 { return }
        guard let client = session.client, let store = session.progress else { failure = "Connect to your Server to play audio."; return }
        do {
            let item = try await client.item(id: itemID)
            try Task.checkCancellation()
            if item.kind == .music {
                let queue = try await client.musicQueue(item: item)
                try Task.checkCancellation()
                try await session.player.playQueue(queue, at: 0, client: client, store: store)
            } else { try await session.player.play(item, client: client, store: store) }
        } catch is CancellationError {} catch { failure = AppSession.message(error) }
    }
    private func perform(_ action: @escaping @MainActor () async throws -> Void) {
        Task { do { try await action() } catch { failure = AppSession.message(error) } }
    }
}

struct AudioLoadingState: View {
    var title = "Opening audio…"
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    private var nowPlayingLayout: AnyLayout { AudioNowPlayingGeometry.layout(accessibility: dynamicTypeSize.isAccessibilitySize) }
    var body: some View {
        nowPlayingLayout {
            RoundedRectangle(cornerRadius: 24).fill(KinoTheme.surface).aspectRatio(1, contentMode: .fit).frame(maxWidth: 440)
            VStack(spacing: 28) {
                VStack(spacing: 8) {
                    RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(width: 260, height: 36)
                    RoundedRectangle(cornerRadius: 5).fill(KinoTheme.raised).frame(width: 160, height: 16)
                }
                Capsule().fill(KinoTheme.raised).frame(height: 5)
                HStack(spacing: AudioNowPlayingGeometry.transportSpacing) {
                    ForEach(0..<3) { index in Circle().fill(KinoTheme.surface)
                        .frame(width: AudioNowPlayingGeometry.transportSize(index), height: AudioNowPlayingGeometry.transportSize(index)) }
                }
                ViewThatFits(in: .horizontal) {
                    HStack(spacing: 12) {
                        ForEach(0..<3) { _ in Capsule().fill(KinoTheme.surface).frame(width: 94, height: 44) }
                    }
                    VStack(spacing: 12) {
                        ForEach(0..<3) { _ in Capsule().fill(KinoTheme.surface).frame(width: 94, height: 44) }
                    }
                }
            }
        }
        .frame(maxWidth: AudioNowPlayingGeometry.width(accessibility: dynamicTypeSize.isAccessibilitySize)).frame(maxWidth: .infinity)
        .skeletonLoading(title)
    }
}

struct MiniPlayer: View {
    @Environment(AppSession.self) private var session
    @State private var expanded = false
    #if os(tvOS)
    @FocusState private var titleFocused: Bool
    #endif
    #if os(iOS)
    private let controlSize: ControlSize = .extraLarge
    private let contentSpacing: CGFloat = 12
    private let horizontalPadding: CGFloat = 20
    private let verticalPadding: CGFloat = 16
    #else
    private let controlSize: ControlSize = .large
    private let contentSpacing: CGFloat = 16
    private let horizontalPadding: CGFloat = 24
    private let verticalPadding: CGFloat = 10
    #endif
    var body: some View {
        if let item = session.player.currentItem, item.isAudio {
            HStack(spacing: contentSpacing) {
                Button { expanded = true } label: {
                    HStack(spacing: 12) {
                        Artwork(path: item.artwork, symbol: "music.note", ratio: 1, dimension: 800).frame(width: 48).clipShape(.rect(cornerRadius: 8))
                        VStack(alignment: .leading) { Text(item.title).font(.headline).lineLimit(1); Text(item.artist).font(.caption).foregroundStyle(.secondary).lineLimit(1) }
                    }.frame(maxWidth: .infinity, alignment: .leading)
                        #if os(tvOS)
                        .padding(12)
                        .overlay { if titleFocused { RoundedRectangle(cornerRadius: 14).strokeBorder(KinoTheme.text, lineWidth: 3) } }
                        #endif
                }
                    #if os(tvOS)
                    .buttonStyle(.borderless).focused($titleFocused)
                    #else
                    .buttonStyle(.plain)
                    #endif
                    .accessibilityLabel("Now playing: \(item.title)")
                Button(session.player.isPlaying || session.player.buffering ? "Pause" : "Play", systemImage: session.player.isPlaying || session.player.buffering ? "pause.fill" : "play.fill") { session.player.togglePlayback() }.labelStyle(.iconOnly).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground().controlSize(controlSize)
                Button("Stop", systemImage: "xmark") { session.player.stop(); session.contentRevision = UUID() }.labelStyle(.iconOnly).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground().controlSize(controlSize)
            }
            .padding(.horizontal, horizontalPadding).padding(.vertical, verticalPadding).background(.regularMaterial)
            #if os(tvOS)
            .fullScreenCover(isPresented: $expanded) { NavigationStack { AudioPlayerScreen(itemID: item.id, hidesMiniPlayer: false) } }
            #else
            .sheet(isPresented: $expanded) { NavigationStack { AudioPlayerScreen(itemID: item.id, hidesMiniPlayer: false) }.presentationSizing(.page) }
            #endif
        }
    }
}
