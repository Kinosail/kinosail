import SwiftUI

struct PlayerTabs: View {
    @AppStorage private var watchStored: String
    @AppStorage private var listenStored: String
    @AppStorage private var modeStored: String
    @State private var selection = PlayerTab.home
    @State private var paths: [PlayerTab: NavigationPath] = [:]
    init(profileKey: String) {
        let legacy = UserDefaults.standard.string(forKey: "kinosail.tabs.\(profileKey)")
        let watch = UserDefaults.standard.string(forKey: PlayerMode.watch.tabsKey(profileKey)) ?? PlayerTab.legacyDefault(legacy)
        let listen = UserDefaults.standard.string(forKey: PlayerMode.listen.tabsKey(profileKey))
            ?? PlayerMode.listen.defaultTabs.map(\.rawValue).joined(separator: ",")
        #if os(iOS)
        let initialMode = PlayerMode.stored(UserDefaults.standard.string(forKey: PlayerMode.storageKey(profileKey)))
        #else
        let initialMode = PlayerMode.watch
        #endif
        let initial = (try? PlayerTab.parse(initialMode == .watch ? watch : listen)) ?? initialMode.defaultTabs
        _selection = State(initialValue: initial.first ?? .home)
        _watchStored = AppStorage(wrappedValue: PlayerTab.legacyDefault(legacy), PlayerMode.watch.tabsKey(profileKey))
        _listenStored = AppStorage(wrappedValue: PlayerMode.listen.defaultTabs.map(\.rawValue).joined(separator: ","), PlayerMode.listen.tabsKey(profileKey))
        _modeStored = AppStorage(wrappedValue: PlayerMode.watch.rawValue, PlayerMode.storageKey(profileKey))
    }
    private var mode: PlayerMode {
        #if os(iOS)
        PlayerMode.stored(modeStored)
        #else
        .watch
        #endif
    }
    private var pinned: [PlayerTab] { (try? PlayerTab.parse(mode == .watch ? watchStored : listenStored)) ?? mode.defaultTabs }
    var body: some View {
        TabView(selection: $selection) {
            ForEach(pinned) { tab in
                #if os(iOS)
                let role: TabRole? = tab == .search ? .search : nil
                #else
                let role: TabRole? = nil
                #endif
                Tab(value: tab, role: role) { stack(tab: tab) { PlayerTabScreen(tab: tab, mode: activeMode, showsSearch: !pinned.contains(.search), changeMode: changeMode) } } label: {
                    Label(tab.title, systemImage: tab.symbol)
                    #if os(tvOS)
                        .foregroundStyle(selection == tab ? KinoTheme.signalInk : KinoTheme.text)
                    #endif
                }
            }
            Tab("More", systemImage: "ellipsis", value: PlayerTab.more) {
                stack(tab: .more) {
                    List {
                        Section {
                            ForEach(PlayerTab.available.filter { !pinned.contains($0) }) { tab in
                                NavigationLink(value: tab) { Label(tab.title, systemImage: tab.symbol) }
                            }
                        }
                        Section { NavigationLink("Customize tabs", value: ScreenDestination.tabPreferences) }
                    }
                    #if os(iOS)
                    .scrollContentBackground(.hidden)
                    #endif
                    .background(KinoTheme.background).navigationTitle("More")
                }
            }
        }
        #if os(iOS)
        .tabViewStyle(.sidebarAdaptable)
        .modifier(MiniPlayerTabAccessory())
        #endif
        .safeAreaInset(edge: .top, spacing: 0) {
            ConnectionBanner {
                let tab: PlayerTab = pinned.contains(.downloads) ? .downloads : .more
                var path = NavigationPath()
                if tab == .more { path.append(PlayerTab.downloads) }
                paths[tab] = path
                selection = tab
            }
        }
        .onAppear { if !pinned.contains(selection) && selection != .more { selection = pinned[0] } }
        .onChange(of: pinned) { _, _ in
            if !pinned.contains(selection) && selection != .more { selection = pinned[0] }
        }
        .onChange(of: mode) { _, _ in selection = pinned[0]; paths = [:] }
    }
    private var activeMode: PlayerMode? {
        #if os(iOS)
        mode
        #else
        nil
        #endif
    }
    private func changeMode(_ next: PlayerMode) { modeStored = next.rawValue }
    private func stack<Content: View>(tab: PlayerTab, @ViewBuilder content: () -> Content) -> some View {
        NavigationStack(path: Binding(get: { paths[tab] ?? NavigationPath() }, set: { paths[tab] = $0 })) {
            content()
                .navigationDestination(for: PlayerTab.self) { PlayerTabScreen(tab: $0, mode: activeMode, showsSearch: !pinned.contains(.search), changeMode: changeMode) }
                #if os(iOS)
                .modifier(SupporterToolbar())
                .toolbar {
                    if tab != .home {
                        ToolbarItem(placement: .topBarTrailing) {
                            Button(mode.other.title) { changeMode(mode.other) }
                                .accessibilityLabel("Switch to \(mode.other.title) mode")
                        }
                    }
                }
                #endif
                .navigationDestination(for: ScreenDestination.self) { DestinationScreen(destination: $0) }
        }
        #if os(tvOS)
        .toolbarBackground(.hidden, for: .navigationBar, .tabBar)
        .toolbarColorScheme(.dark, for: .navigationBar, .tabBar)
        #endif
    }
}

#if os(iOS)
private struct MiniPlayerTabAccessory: ViewModifier {
    @Environment(AppSession.self) private var session

    @ViewBuilder func body(content: Content) -> some View {
        if #available(iOS 26.1, *) {
            content.tabViewBottomAccessory(isEnabled: session.player.currentItem?.isAudio == true) { MiniPlayer() }
        } else {
            content.tabViewBottomAccessory { MiniPlayer() }
        }
    }
}
#endif

private struct PlayerTabScreen: View {
    let tab: PlayerTab
    let mode: PlayerMode?
    let showsSearch: Bool
    let changeMode: (PlayerMode) -> Void
    var body: some View {
        switch tab {
        case .movies: LibraryScreen(initialView: .movies)
        case .shows: LibraryScreen(initialView: .shows)
        case .home: HomeScreen(showsSearch: showsSearch, mode: mode, changeMode: mode == nil ? nil : changeMode)
        case .search: LibraryScreen(initialView: mode?.searchViews.first ?? .all, searchMode: true, mode: mode).id(mode)
        case .list: LibraryScreen(initialView: .list)
        case .library: LibraryHubScreen(mode: mode)
        case .music: MusicScreen()
        case .audiobooks: LibraryScreen(initialView: .audiobooks)
        case .books: LibraryScreen(initialView: .books)
        case .photos: LibraryScreen(initialView: .photos)
        case .collections: CollectionsScreen()
        case .downloads: DownloadsScreen()
        case .settings: SettingsScreen()
        case .more: EmptyView()
        }
    }
}
