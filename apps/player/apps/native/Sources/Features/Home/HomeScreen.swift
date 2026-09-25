import SwiftUI

struct HomeScreen: View {
    var showsSearch = true
    var mode: PlayerMode?
    var changeMode: ((PlayerMode) -> Void)?
    @Environment(AppSession.self) private var session
    #if os(tvOS)
    @Namespace private var homeFocus
    @State private var quickPlay: ScreenDestination?
    #endif
    var body: some View {
        ScrollView {
            HStack(spacing: 28) {
                Text("For you").font(.title2.bold()).frame(minHeight: 44)
                    .accessibilityAddTraits(.isHeader)
                NavigationLink("My List", value: ScreenDestination.library(.list))
                    .font(.callout).frame(minHeight: 44)
                    #if os(tvOS)
                    .buttonStyle(.bordered).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
                    #else
                    .foregroundStyle(KinoTheme.muted)
                    #endif
            }
            .frame(maxWidth: .infinity, alignment: .leading)
            .padding(.horizontal, KinoTheme.contentPadding)

            // Refresh belongs to a vertical scroll container, not the nested media shelf.
            ResourceView(identity: "\(session.profileKey ?? ""):\(mode?.rawValue ?? "all")", refreshID: session.contentRevision.uuidString, loadingLayout: .home, allowsPullToRefresh: false, load: { policy in
                guard let client = session.client else { throw ClientError.http(401) }
                return try await client.home(mode: mode, policy: policy)
            }) { home in
                #if os(tvOS)
                let selection = HomeSelection(continueWatching: home.continueWatching.filter { $0.kind != .book },
                                              recent: home.recent.filter { $0.kind != .book })
                #else
                let selection = HomeSelection(continueWatching: home.continueWatching, recent: home.recent, mode: mode)
                #endif
                VStack(alignment: .leading, spacing: 32) {
                    if let featured = selection.featured {
                        CinemaHero(item: featured, showsPlot: false) {
                            NavigationLink(value: featured.playingDestination) {
                                Label(featured.playLabel, systemImage: featured.kind == .book ? "book.fill" : "play.fill").frame(maxWidth: .infinity)
                            }.buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                            #if os(tvOS)
                            .tvOSDefaultPlayFocus(in: homeFocus, id: "home.play.\(featured.id)", enabled: featured.kind == .video || featured.isAudio)
                            #endif
                            NavigationLink("Details", value: featured.destination).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
                        }
                    }
                    if !selection.continuation.isEmpty {
                        ResumeRows(items: selection.continuation, title: mode == .listen ? "Continue listening" : "Continue watching",
                                   showsAll: mode != .listen)
                    }
                    if !selection.recent.isEmpty {
                        #if os(tvOS)
                        MediaShelf(title: "Recently added", items: selection.recent, onQuickPlay: { quickPlay = $0 })
                        #else
                        MediaShelf(title: mode == .listen ? "Music & audiobooks" : mode == .watch ? "Movies & shows" : "Recently added",
                                   items: selection.recent)
                        #endif
                    }
                    if selection.featured == nil {
                        FeaturePlaceholder(title: mode == .listen ? "Nothing to listen to yet" : "Your library is ready",
                                           symbol: mode == .listen ? "headphones" : "play.rectangle",
                                           message: mode == .listen ? "Add music or audiobooks to your Server to see them here." : "Media added to your Server will appear here.")
                    }
                    VStack(alignment: .leading, spacing: 12) {
                        Text("Browse library").font(.title2.bold()).accessibilityAddTraits(.isHeader)
                        LibraryQuickLinks(mode: mode)
                    }
                }
            }
            .padding([.horizontal, .bottom], KinoTheme.contentPadding)
            .padding(.top, 12)
        }
        .refreshable {
            await session.client?.invalidateCatalog()
            session.contentRevision = UUID()
        }
        .cinemaBackground()
        #if os(tvOS)
        .focusScope(homeFocus)
        .navigationTitle("")
        .navigationDestination(item: $quickPlay) { DestinationScreen(destination: $0) }
        #else
        .navigationTitle("Kinosail")
        #endif
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar {
            if let mode, let changeMode {
                ToolbarItem(placement: .topBarTrailing) {
                    Button(mode.other.title) { changeMode(mode.other) }
                        .accessibilityLabel("Switch to \(mode.other.title) mode")
                }
            }
            if showsSearch {
                ToolbarItem {
                    if mode == nil {
                        NavigationLink(value: ScreenDestination.search) { Image(systemName: "magnifyingglass").accessibilityLabel("Search library") }
                    } else if let mode {
                        NavigationLink(value: PlayerTab.search) { Image(systemName: "magnifyingglass").accessibilityLabel("Search \(mode.title) mode") }
                    }
                }
            }
        }
    }
}

struct HomeSelection {
    let featured: MediaItem?
    let continuation: [MediaItem]
    let recent: [MediaItem]

    init(continueWatching: [MediaItem], recent: [MediaItem], mode: PlayerMode? = nil) {
        let watching = continueWatching.filter { mode?.includes($0) ?? true }
        let added = recent.filter { mode?.includes($0) ?? true }
        featured = watching.first ?? added.first
        continuation = Array(watching.dropFirst().prefix(4))
        let visibleIDs = Set(([featured].compactMap { $0 } + continuation).map(\.id))
        self.recent = added.filter { !visibleIDs.contains($0.id) }
    }
}
