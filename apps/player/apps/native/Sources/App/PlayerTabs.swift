import SwiftUI

struct PlayerTabs: View {
    @AppStorage private var stored: String
    @State private var selection = PlayerTab.home
    @State private var paths: [PlayerTab: NavigationPath] = [:]
    init(profileKey: String) {
        let legacy = UserDefaults.standard.string(forKey: "kinosail.tabs.\(profileKey)")
        let storageKey = "kinosail.tabs.v2.\(profileKey)"
        let raw = UserDefaults.standard.string(forKey: storageKey) ?? PlayerTab.legacyDefault(legacy)
        let initial = (try? PlayerTab.parse(raw)) ?? PlayerTab.defaults
        _selection = State(initialValue: initial.first ?? .home)
        _stored = AppStorage(wrappedValue: PlayerTab.legacyDefault(legacy), storageKey)
    }
    private var pinned: [PlayerTab] { (try? PlayerTab.parse(stored)) ?? PlayerTab.defaults }
    var body: some View {
        TabView(selection: $selection) {
            ForEach(pinned) { tab in
                #if os(iOS)
                let role: TabRole? = tab == .search ? .search : nil
                #else
                let role: TabRole? = nil
                #endif
                Tab(value: tab, role: role) { stack(tab: tab) { PlayerTabScreen(tab: tab, showsSearch: !pinned.contains(.search)) } } label: {
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
        .onChange(of: stored) { _, _ in
            if !pinned.contains(selection) && selection != .more { selection = pinned[0] }
        }
    }
    private func stack<Content: View>(tab: PlayerTab, @ViewBuilder content: () -> Content) -> some View {
        NavigationStack(path: Binding(get: { paths[tab] ?? NavigationPath() }, set: { paths[tab] = $0 })) {
            content()
                .navigationDestination(for: PlayerTab.self) { PlayerTabScreen(tab: $0, showsSearch: !pinned.contains(.search)) }
                .modifier(SupporterToolbar())
                .navigationDestination(for: ScreenDestination.self) { DestinationScreen(destination: $0) }
        }
        #if os(tvOS)
        .toolbarBackground(.hidden, for: .navigationBar, .tabBar)
        .toolbarColorScheme(.dark, for: .navigationBar, .tabBar)
        #endif
    }
}

private struct PlayerTabScreen: View {
    let tab: PlayerTab
    let showsSearch: Bool
    var body: some View {
        switch tab {
        case .movies: LibraryScreen(initialView: .movies)
        case .shows: LibraryScreen(initialView: .shows)
        case .home: HomeScreen(showsSearch: showsSearch)
        case .search: LibraryScreen(searchMode: true)
        case .list: LibraryScreen(initialView: .list)
        case .library: LibraryHubScreen()
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
