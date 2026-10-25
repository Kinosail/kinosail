import AVKit
import SwiftUI

struct PlaybackScreen: View {
    let itemID: String
    var onClose: (() -> Void)? = nil
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var failure: String?
    @State private var revision = 0
    @State private var showsTools = false
    @State private var showsSeekPreview = false
    #if os(iOS) || os(tvOS)
    @State private var showsFullScreen = false
    @State private var didPresentFullScreen = false
    #endif

    var body: some View {
        #if os(iOS)
        if onClose == nil {
            fullScreenPlayback
        } else {
            playbackContent
        }
        #elseif os(tvOS)
        if onClose == nil { fullScreenPlayback }
        else { playbackContent }
        #else
        playbackContent
        #endif
    }

    #if os(iOS) || os(tvOS)
    private var fullScreenPlayback: some View {
        Color.clear
            .fullScreenCover(isPresented: $showsFullScreen, onDismiss: { dismiss() }) {
                NavigationStack {
                    PlaybackScreen(itemID: itemID, onClose: { showsFullScreen = false })
                }
            }
            .onAppear {
                guard !didPresentFullScreen else { return }
                didPresentFullScreen = true
                showsFullScreen = true
            }
    }
    #endif

    private var playbackContent: some View {
        Group {
            #if os(iOS)
            TouchPlaybackView(failure: failure, retry: { revision += 1 }, close: { closePlayback() })
            #else
            TVPlaybackSurface(failure: failure, retry: { revision += 1 }, close: { closePlayback() },
                              options: { showsTools = true }, seekPreview: { showsSeekPreview = true })
            #endif
        }
        .navigationTitle(session.player.currentItem?.title ?? "Playback")
        .toolbar(.hidden, for: .navigationBar, .tabBar)
        #if os(tvOS)
        .fullScreenCover(isPresented: $showsTools) { NavigationStack { PlaybackToolsScreen() }.tvPlaybackPanel() }
        .sheet(isPresented: $showsSeekPreview) { TVSeekPreviewScreen() }
        #else
        .sheet(isPresented: $showsTools) { NavigationStack { PlaybackToolsScreen() } }
        #endif
        .task(id: "\(itemID):\(revision)") {
            failure = nil
            guard let client = session.client, let store = session.progress else { failure = "Connect to your Server to play this title."; return }
            if session.player.currentItem?.id == itemID, session.player.player != nil || session.player.loading, revision == 0 { return }
            do {
                let item: MediaItem
                if let cached = try? await client.item(id: itemID, policy: .cached) { item = cached }
                else { item = try await client.item(id: itemID, policy: .automatic) }
                try Task.checkCancellation()
                try await session.player.play(item, client: client, store: store)
            } catch is CancellationError {} catch { failure = AppSession.message(error) }
        }
        .onDisappear {
            if !session.player.presentation.pictureInPicture, !showsTools, !showsSeekPreview, !session.player.presentation.showingOptions,
               session.player.currentItem?.kind == .video { session.player.stop(); session.contentRevision = UUID() }
        }
    }

    private func closePlayback() {
        if !session.player.presentation.pictureInPicture, session.player.currentItem?.kind == .video { session.player.pause() }
        if let onClose { onClose() } else { dismiss() }
    }
}

#if os(tvOS)
struct TVPlaybackSurface: View {
    let failure: String?
    let retry: () -> Void
    let close: () -> Void
    var options: () -> Void = {}
    var seekPreview: () -> Void = {}
    @Environment(AppSession.self) private var session
    @State private var mounted = false

    private var message: String? {
        if let failure { return failure }
        if let message = session.player.message, !session.player.recoveringNetwork { return message }
        return nil
    }

    var body: some View {
        let message = message
        let player = session.player.player
        ZStack {
            if mounted || player != nil {
                NativePlayerView(player: player, presentation: session.player.presentation,
                                 options: options, seekPreview: seekPreview, close: close,
                                 restore: { session.showsVideoPlayer = true }, interactive: message == nil)
                    .onExitCommand { session.player.presentation.requestClose() }
                    .background(.black)
                    .ignoresSafeArea()
                    .accessibilityHidden(message != nil || player == nil)
                    .onAppear { mounted = true }
                    .background { TVPlaybackPromptActions() }
            }
            if let message {
                ZStack {
                    Color.black.ignoresSafeArea()
                    RetryState(message: message, focusesRetry: true, retry: retry)
                }
                .onExitCommand { close() }
            } else if player == nil {
                ZStack {
                    Color.black.ignoresSafeArea()
                    Text("Opening video…").foregroundStyle(.white)
                }
                .frame(maxWidth: .infinity, maxHeight: .infinity)
                .onExitCommand { close() }
            }
        }
    }
}

struct NativePlayerView: UIViewControllerRepresentable {
    let player: AVPlayer?
    let presentation: PlayerPresentation
    let options: () -> Void
    let seekPreview: () -> Void
    let close: () -> Void
    let restore: () -> Void
    var interactive = true

    func makeUIViewController(context: Context) -> AVPlayerViewController {
        presentation.restore = restore
        presentation.showOptions = options
        presentation.showSeekPreview = seekPreview
        presentation.close = close
        presentation.controller.player = player
        presentation.controller.view.isUserInteractionEnabled = interactive
        presentation.appeared()
        DispatchQueue.main.async { presentation.isolateRemoteTouches() }
        return presentation.controller
    }
    func updateUIViewController(_ controller: AVPlayerViewController, context: Context) {
        presentation.restore = restore
        presentation.showOptions = options
        presentation.showSeekPreview = seekPreview
        presentation.close = close
        if controller.player !== player { controller.player = player }
        if controller.view.isUserInteractionEnabled != interactive { controller.view.isUserInteractionEnabled = interactive }
        if interactive { DispatchQueue.main.async { presentation.isolateRemoteTouches() } }
        else { presentation.restoreRemoteTouches() }
    }
    func makeCoordinator() -> PlayerPresentation { presentation }
    static func dismantleUIViewController(_ controller: AVPlayerViewController, coordinator: PlayerPresentation) {
        coordinator.visible = false
        coordinator.restoreRemoteTouches()
        coordinator.close = nil
        if !coordinator.pictureInPicture { controller.player = nil }
    }
}
#endif

struct PlaybackToolsScreen: View {
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var message: String?
    #if os(tvOS)
    @Namespace private var playbackToolsFocus
    #endif

    var body: some View {
        List {
            if let item = session.player.currentItem {
                #if os(tvOS)
                TVPlaybackOptionRows(item: item)
                if let message { Text(message).foregroundStyle(.secondary) }
                #else
                Section {
                    Text(sourceStatus).foregroundStyle(.secondary)
                    if let progress = session.player.progressMessage { Text(progress) }
                    NavigationLink("This title’s preferences") { PlaybackPreferencesScreen(itemID: item.id) }
                    if !session.usesDeviceLibrary {
                        NavigationLink("Bookmarks", value: ScreenDestination.bookmarks(item.id))
                        #if os(iOS)
                        NavigationLink("Play on another device", value: ScreenDestination.playOnTV(item.id))
                        #endif
                    }
                    NavigationLink("Playback speed") { PlaybackSpeedScreen() }
                } header: {
                    #if !os(tvOS)
                    Text(item.title)
                    #endif
                }
                if let message { Text(message).foregroundStyle(.secondary) }
                PlaybackTrackSections()
                #endif
                if let source = session.player.source {
                    #if !os(tvOS)
                    if !source.chapters.isEmpty {
                        Section("Chapters") { ForEach(source.chapters) { chapter in Button("\(chapter.title) · \(chapter.start.clock)") { perform { try await session.player.seek(to: chapter.start); dismiss() } } } }
                    }
                    #endif
                    if !source.markers.isEmpty {
                        Section("Skip a section") { ForEach(source.markers) { marker in Button("Skip \(marker.label.isEmpty ? marker.type : marker.label)") { perform { try await session.player.seek(to: marker.end); dismiss() } } } }
                    }
                    if let next = source.nextItemID {
                        Section { Button("Play next episode") { perform {
                            guard let client = session.client, let store = session.progress else { throw ClientError.unavailable }
                            try await session.player.play(client.item(id: next), client: client, store: store)
                            dismiss()
                        } }
                        #if os(tvOS)
                        .tvOSDefaultPlayFocus(in: playbackToolsFocus, id: "playback.next.\(next)")
                        #endif
                        }
                    }
                }
            }
        }
        .tvOSConfigurationLayout(title: session.player.currentItem?.title ?? "Playback options", symbol: "play.circle",
                                 id: "configuration.Playback options")
        .configurationNavigationTitle("Playback options")
        #if os(tvOS)
        .focusScope(playbackToolsFocus)
        #else
        .toolbar { ToolbarItem(placement: .cancellationAction) { Button("Done") { dismiss() } } }
        #endif
        .navigationDestination(for: ScreenDestination.self) { DestinationScreen(destination: $0) }
    }
    private var sourceStatus: String {
        session.player.usingCompatibility ? "Playing a compatible version" : "Playing the original file"
    }
    private func perform(_ action: @escaping @MainActor () async throws -> Void) {
        Task { do { try await action(); message = nil } catch { message = AppSession.message(error) } }
    }
}
