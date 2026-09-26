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
            // Refresh belongs to a vertical scroll container, not the nested media shelf.
            ResourceView(identity: "\(session.profileKey ?? ""):\(mode?.rawValue ?? "all")", refreshID: session.contentRevision.uuidString, loadingLayout: mode == .listen ? .homeAudio : .home, allowsPullToRefresh: false, load: { policy in
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
                    VStack(alignment: .leading, spacing: 12) {
                        HStack(spacing: 28) {
                            Text(selection.featuredIsContinuing ? (mode == .listen ? "Listening" : "Watching") : "For you")
                                .font(.title2.bold()).frame(minHeight: 44).accessibilityAddTraits(.isHeader)
                            NavigationLink("My List", value: ScreenDestination.library(.list))
                                .font(.callout).frame(minHeight: 44)
                                #if os(tvOS)
                                .buttonStyle(.bordered).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
                                #else
                                .foregroundStyle(KinoTheme.muted)
                                #endif
                            #if os(tvOS)
                            Spacer()
                            if let mode, let changeMode {
                                Button { changeMode(mode.other) } label: {
                                    Label(mode.other.title, systemImage: mode.other == .listen ? "headphones" : "tv")
                                }
                                .buttonStyle(.bordered).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
                                .accessibilityLabel("Switch to \(mode.other.title) mode")
                            }
                            #endif
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                        if let featured = selection.featured {
                            CinemaHero(item: featured, subtitle: homeSubtitle(for: featured), showsPlot: false) {
                                NavigationLink(value: featured.playingDestination) {
                                    Label(featured.playLabel, systemImage: featured.kind == .book ? "book.fill" : "play.fill")
                                        #if os(tvOS)
                                        .frame(minWidth: 280).padding(.vertical, 12)
                                        .foregroundStyle(KinoTheme.tvOSPrimaryInk)
                                        .background(KinoTheme.tvOSPrimaryFill, in: Capsule())
                                        #else
                                        .frame(maxWidth: .infinity)
                                        #endif
                                }
                                #if os(tvOS)
                                .buttonStyle(.card)
                                .tvOSDefaultPlayFocus(in: homeFocus, id: "home.play.\(featured.id)", enabled: featured.kind == .video || featured.isAudio)
                                #else
                                .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                                #endif
                                NavigationLink("Details", value: featured.destination).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
                            }
                        }
                    }
                    if !selection.continuation.isEmpty {
                        if mode == .listen {
                            ResumeRows(items: selection.continuation, title: "Continue listening", showsAll: false)
                        } else {
                            MediaShelf(title: "Up Next", items: selection.continuation, landscape: true, resumesPlayback: true,
                                       moreTitle: "See all", moreDestination: .library(.history))
                        }
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
                    #if os(iOS)
                    VStack(alignment: .leading, spacing: 12) {
                        Text("Browse library").font(.title2.bold()).accessibilityAddTraits(.isHeader)
                        LibraryQuickLinks(mode: mode)
                    }
                    #endif
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
            #if os(iOS)
            if let mode, let changeMode {
                ToolbarItem(placement: .topBarTrailing) {
                    Button(mode.other.title) { changeMode(mode.other) }
                        .accessibilityLabel("Switch to \(mode.other.title) mode")
                }
            }
            #endif
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

    private func homeSubtitle(for item: MediaItem) -> String {
        #if os(tvOS)
        item.subtitleWithoutYear
        #else
        item.subtitle
        #endif
    }
}

struct HomeSelection {
    let featured: MediaItem?
    let featuredIsContinuing: Bool
    let continuation: [MediaItem]
    let recent: [MediaItem]

    init(continueWatching: [MediaItem], recent: [MediaItem], mode: PlayerMode? = nil) {
        let watching = continueWatching.filter { mode?.includes($0) ?? true }
        let added = recent.filter { mode?.includes($0) ?? true }
        featured = watching.first ?? added.first
        featuredIsContinuing = !watching.isEmpty
        continuation = Array(watching.dropFirst().prefix(4))
        let visibleIDs = Set(([featured].compactMap { $0 } + continuation).map(\.id))
        self.recent = added.filter { !visibleIDs.contains($0.id) }
    }
}
