import SwiftUI

private enum AudioNowPlayingGeometry {
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
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @Environment(\.dynamicTypeSize) private var dynamicTypeSize
    @State private var failure: String?
    @State private var revision = 0
    @State private var seekValue: Double = 0
    @State private var seeking = false
    #if os(tvOS)
    @State private var showsTools = false
    @Namespace private var audioFocus
    #endif
    private var upcoming: [MediaItem] { Array(session.player.queue.items.dropFirst((session.player.queue.currentIndex ?? 0) + 1).prefix(20)) }
    private var nowPlayingLayout: AnyLayout { AudioNowPlayingGeometry.layout(accessibility: dynamicTypeSize.isAccessibilitySize) }
    var body: some View {
        ScrollView {
            if let failure { RetryState(message: failure) { revision += 1 } }
            else if let message = session.player.message { RetryState(message: message) { revision += 1 } }
            else if let item = session.player.currentItem, item.isAudio {
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
                            Slider(value: $seekValue, in: 0...max(1, session.player.duration), onEditingChanged: { editing in
                                seeking = editing
                                if !editing { perform { try await session.player.seek(to: seekValue) } }
                            }).disabled(session.player.duration <= 0).accessibilityLabel("Playback position").accessibilityValue("\(seekValue.clock) of \(session.player.duration.clock)")
                            #else
                            ProgressView(value: min(session.player.seconds, session.player.duration), total: max(1, session.player.duration))
                                .tint(KinoTheme.signal)
                                .accessibilityLabel("Playback position")
                                .accessibilityValue("\(session.player.seconds.clock) of \(session.player.duration.clock)")
                            #endif
                            HStack { Text(session.player.seconds.clock); Spacer(); Text(session.player.duration.clock) }.font(.caption.monospacedDigit()).foregroundStyle(.secondary)
                        }
                        HStack(spacing: 12) {
                            Button("Back 15 seconds", systemImage: "gobackward.15") { perform { try await session.player.seek(to: max(0, session.player.seconds - 15)) } }.labelStyle(.iconOnly).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text).controlSize(.large)
                            Button(session.player.isPlaying ? "Pause" : "Play", systemImage: session.player.isPlaying ? "pause.fill" : "play.fill") { session.player.togglePlayback() }
                                .labelStyle(.iconOnly).font(.largeTitle).buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                                #if os(tvOS)
                                .tvOSDefaultPlayFocus(in: audioFocus, id: "audio.play.\(item.id)", enabled: session.player.player != nil)
                                #endif
                            Button("Forward 30 seconds", systemImage: "goforward.30") { perform { try await session.player.seek(to: min(session.player.duration, session.player.seconds + 30)) } }.labelStyle(.iconOnly).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text).controlSize(.large)
                        }.font(.title2).disabled(session.player.player == nil)
                        if item.kind == .music {
                            HStack(spacing: 24) {
                                Button("Previous", systemImage: "backward.end.fill") { perform { try await session.player.previousTrack() } }
                                Button("Next", systemImage: "forward.end.fill") { perform { try await session.player.nextTrack() } }
                            }.labelStyle(.iconOnly).font(.title2).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text).controlSize(.large).disabled(session.player.player == nil)
                        }
                        ViewThatFits(in: .horizontal) {
                            HStack(spacing: 12) { audioOptions(item) }
                            VStack(spacing: 12) { audioOptions(item) }
                        }.font(.callout)
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
        .navigationTitle("Now playing")
        #if os(tvOS)
        .focusScope(audioFocus)
        .onPlayPauseCommand { if session.player.player != nil { session.player.togglePlayback() } }
        #endif
        .toolbar { ToolbarItem(placement: .primaryAction) {
            #if os(tvOS)
            Button("Playback options", systemImage: "ellipsis.circle") { showsTools = true }
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
            Menu("Repeat: \(session.player.queue.repeatMode.rawValue)", systemImage: "repeat") {
                ForEach(MediaQueue.RepeatMode.allCases, id: \.self) { mode in Button(mode.rawValue.capitalized) { session.player.queue.setRepeat(mode) } }
            }
        }
        Menu("Sleep timer", systemImage: "moon") {
            ForEach([15, 30, 45, 60, 90], id: \.self) { minutes in
                Button("\(minutes) minutes") { perform { try session.player.setSleepTimer(deadline: Date().addingTimeInterval(Double(minutes) * 60)) } }
            }
            Button("Turn off") { perform { try session.player.setSleepTimer(deadline: nil) } }
        }
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
                HStack(spacing: 12) {
                    ForEach(0..<3) { _ in Circle().fill(KinoTheme.surface).frame(width: 52, height: 52) }
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
                }.buttonStyle(.plain).accessibilityLabel("Now playing: \(item.title)")
                Button(session.player.isPlaying ? "Pause" : "Play", systemImage: session.player.isPlaying ? "pause.fill" : "play.fill") { session.player.togglePlayback() }.labelStyle(.iconOnly).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text).controlSize(controlSize)
                Button("Stop", systemImage: "xmark") { session.player.stop(); session.contentRevision = UUID() }.labelStyle(.iconOnly).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text).controlSize(controlSize)
            }
            .padding(.horizontal, horizontalPadding).padding(.vertical, verticalPadding).background(.regularMaterial)
            .sheet(isPresented: $expanded) { NavigationStack { AudioPlayerScreen(itemID: item.id) }.presentationSizing(.page) }
        }
    }
}
