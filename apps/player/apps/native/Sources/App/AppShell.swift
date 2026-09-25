import SwiftUI

enum ScreenDestination: Hashable {
    case search, library(LibraryView), detail(String), show(String), actor(String), collections, collection(String)
    case music, album(String), audio(String), playback(String), photos(String)
    case reader(String), downloads, offlinePlayback(String), approval, playbackPreferences, readerPreferences
    case tabPreferences, offlinePreferences, bookmarks(String), progressSync, playOnTV(String)
}

struct AppShell: View {
    @Environment(AppSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase

    var body: some View {
        @Bindable var session = session
        Group {
            if session.restoring {
                FeaturePlaceholder(title: "Opening your library", symbol: "play.rectangle", message: "Signing you back in…")
            } else if session.client == nil {
                SetupScreen()
            } else { tabs.id(session.profileKey) }
        }
            #if os(iOS)
            .scrollContentBackground(.hidden)
            #endif
            .background(KinoTheme.background)
            .fullScreenCover(isPresented: $session.showsVideoPlayer) {
                if let item = session.player.currentItem {
                    NavigationStack {
                        PlaybackScreen(itemID: item.id, onClose: { session.showsVideoPlayer = false })
                    }
                }
            }
            #if os(tvOS)
            .fullScreenCover(isPresented: $session.showsSetup) { SetupScreen().background(KinoTheme.background.ignoresSafeArea()) }
            #else
            .sheet(isPresented: $session.showsSetup) { SetupScreen() }
            #endif
            .sheet(isPresented: Binding(
                get: { session.pendingApprovalCode != nil && session.player.currentItem == nil && !session.reading },
                set: { if !$0 { session.dismissApproval() } }
            )) {
                NavigationStack { ApprovalScreen(initialCode: session.pendingApprovalCode ?? "") }
            }
            .alert("Kinosail", isPresented: Binding(
                get: { session.notice != nil }, set: { if !$0 { session.notice = nil } }
            )) {
                Button("OK") { session.notice = nil }
            } message: { Text(session.notice ?? "") }
            .fullScreenCover(item: $session.pendingMediaLink) { link in
                NavigationStack { MediaLinkScreen(link: link) }.id(link.id)
            }
            .task { await session.restore() }
            .modifier(ConnectionMonitoring())
            .task(id: "\(session.client?.identity.uuidString ?? ""):\(session.restoring):\(scenePhase)") {
                guard scenePhase == .active, !session.restoring, let client = session.client else { return }
                // Visible requests take priority over background catalog warmup.
                do { try await Task.sleep(for: .seconds(1)) }
                catch { return }
                let mode = PlayerMode.stored(UserDefaults.standard.string(forKey: PlayerMode.storageKey(session.profileKey ?? "")))
                let savedTabs = UserDefaults.standard.string(forKey: mode.other.tabsKey(session.profileKey ?? ""))
                let landingTab = savedTabs.flatMap { try? PlayerTab.parse($0).first } ?? .home
                await client.warmCatalog(mode: mode, landingTab: landingTab)
            }
            .task(id: scenePhase) { if scenePhase == .active { await session.casting.monitor() } }
            #if os(iOS)
            .task(id: scenePhase) { if scenePhase == .active { await session.downloads.monitor() } }
            .onChange(of: session.player.isPlaying || session.player.loading || session.player.buffering) { _, active in session.downloads.setPlaybackActive(active) }
            #endif
            .onChange(of: scenePhase) { _, phase in if phase != .active { session.player.checkpoint() } }
    }

    private var tabs: some View {
        #if os(iOS)
        PlayerTabs(profileKey: session.profileKey ?? "")
        #else
        PlayerTabs(profileKey: session.profileKey ?? "")
            .safeAreaInset(edge: .bottom, spacing: 0) { MiniPlayer() }
        #endif
    }

}

struct DestinationScreen: View {
    let destination: ScreenDestination
    var body: some View {
        switch destination {
        case .search: LibraryScreen(searchMode: true)
        case .library(let view): LibraryScreen(initialView: view)
        case .detail(let id): DetailScreen(itemID: id)
        case .show(let id): ShowScreen(showID: id)
        case .actor(let name): ActorScreen(name: name)
        case .collections: CollectionsScreen()
        case .collection(let name): CollectionScreen(name: name)
        case .music: MusicScreen()
        case .album(let id): AlbumScreen(albumID: id)
        case .audio(let id): AudioPlayerScreen(itemID: id)
        case .playback(let id): PlaybackScreen(itemID: id)
        case .photos(let id): PhotoScreen(itemID: id)
        case .reader(let id): ReaderScreen(itemID: id)
        case .downloads: DownloadsScreen()
        case .offlinePlayback(let id): OfflinePlaybackScreen(downloadID: id)
        case .approval: ApprovalScreen()
        case .playbackPreferences: PlaybackPreferencesScreen()
        case .readerPreferences: ReaderPreferencesScreen()
        case .tabPreferences: TabPreferencesScreen()
        case .offlinePreferences: OfflinePreferencesScreen()
        case .bookmarks(let id): BookmarksScreen(itemID: id)
        case .progressSync: ProgressSyncScreen()
        case .playOnTV(let id): PlayOnTVScreen(itemID: id)
        }
    }
}
