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

    var body: some View {
        Group {
            #if os(iOS)
            TouchPlaybackView(failure: failure, retry: { revision += 1 }, close: { if let onClose { onClose() } else { dismiss() } })
            #else
            if let failure { RetryState(message: failure) { revision += 1 } }
            else if let message = session.player.message, !session.player.recoveringNetwork { RetryState(message: message) { revision += 1 } }
            else if let player = session.player.player {
                NativePlayerView(player: player, presentation: session.player.presentation,
                                 options: { showsTools = true }, restore: { session.showsVideoPlayer = true })
                    .overlay {
                        if !session.player.presentation.readyForDisplay {
                            ProgressView("Opening your movie…")
                                .tint(.white).foregroundStyle(.white)
                                .padding().background(.black.opacity(0.8), in: RoundedRectangle(cornerRadius: 12))
                                .allowsHitTesting(false)
                        }
                    }
                    .background(.black)
                    #if os(tvOS)
                    .ignoresSafeArea()
                    #else
                    .ignoresSafeArea(edges: .bottom)
                    #endif
            } else { LoadingState(title: "Opening video…").padding(KinoTheme.contentPadding) }
            #endif
        }
        .navigationTitle(session.player.currentItem?.title ?? "Playback")
        .toolbar(.hidden, for: .navigationBar, .tabBar)
        .sheet(isPresented: $showsTools) { NavigationStack { PlaybackToolsScreen() } }
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
            if !session.player.presentation.pictureInPicture, !showsTools, !session.player.presentation.showingOptions,
               session.player.currentItem?.kind == .video { session.player.stop(); session.contentRevision = UUID() }
        }
    }
}

#if os(tvOS)
struct NativePlayerView: UIViewControllerRepresentable {
    let player: AVPlayer
    let presentation: PlayerPresentation
    let options: () -> Void
    let restore: () -> Void

    func makeUIViewController(context: Context) -> AVPlayerViewController {
        presentation.restore = restore
        presentation.controller.player = player
        #if os(tvOS)
        presentation.controller.transportBarCustomMenuItems = [UIAction(title: "Playback options", image: UIImage(systemName: "ellipsis.circle")) { _ in options() }]
        #endif
        presentation.appeared()
        return presentation.controller
    }
    func updateUIViewController(_ controller: AVPlayerViewController, context: Context) {
        presentation.restore = restore
        if controller.player !== player { controller.player = player }
        #if os(tvOS)
        controller.transportBarCustomMenuItems = [UIAction(title: "Playback options", image: UIImage(systemName: "ellipsis.circle")) { _ in options() }]
        #endif
    }
    func makeCoordinator() -> PlayerPresentation { presentation }
    static func dismantleUIViewController(_ controller: AVPlayerViewController, coordinator: PlayerPresentation) {
        coordinator.visible = false
        if !coordinator.pictureInPicture { controller.player = nil }
    }
}
#endif

struct PlaybackToolsScreen: View {
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var message: String?

    var body: some View {
        List {
            if let item = session.player.currentItem {
                Section(item.title) {
                    Text(session.player.usingCompatibility ? "Playing a compatible version" : "Playing the original file").foregroundStyle(.secondary)
                    if let progress = session.player.progressMessage { Text(progress) }
                    NavigationLink("This title’s preferences") { PlaybackPreferencesScreen(itemID: item.id) }
                    NavigationLink("Bookmarks", value: ScreenDestination.bookmarks(item.id))
                    NavigationLink("Play on TV", value: ScreenDestination.playOnTV(item.id))
                    NavigationLink("Playback speed") { PlaybackSpeedScreen() }
                }
                if let message { Text(message).foregroundStyle(.secondary) }
                PlaybackTrackSections()
                if let source = session.player.source {
                    if !source.chapters.isEmpty {
                        Section("Chapters") { ForEach(source.chapters) { chapter in Button("\(chapter.title) · \(chapter.start.clock)") { perform { try await session.player.seek(to: chapter.start); dismiss() } } } }
                    }
                    if !source.markers.isEmpty {
                        Section("Skip a section") { ForEach(source.markers) { marker in Button("Skip \(marker.label.isEmpty ? marker.type : marker.label)") { perform { try await session.player.seek(to: marker.end); dismiss() } } } }
                    }
                    if let next = source.nextItemID {
                        Section { Button("Play next episode") { perform {
                            guard let client = session.client, let store = session.progress else { throw ClientError.unavailable }
                            try await session.player.play(client.item(id: next), client: client, store: store)
                            dismiss()
                        } } }
                    }
                }
            }
        }
        .navigationTitle("Playback options")
        .toolbar { ToolbarItem(placement: .cancellationAction) { Button("Done") { dismiss() } } }
        .navigationDestination(for: ScreenDestination.self) { DestinationScreen(destination: $0) }
    }
    private func perform(_ action: @escaping @MainActor () async throws -> Void) {
        Task { do { try await action(); message = nil } catch { message = AppSession.message(error) } }
    }
}
