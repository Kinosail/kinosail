import SwiftUI

struct HomeScreen: View {
    var showsSearch = true
    var mode: PlayerMode?
    var changeMode: ((PlayerMode) -> Void)?
    var selectTab: (PlayerTab) -> Void
    var focusTopBar: (() -> Void)? = nil
    var launchFocus = true
    @Environment(AppSession.self) private var session
    #if os(tvOS)
    @State private var quickPlay: ScreenDestination?
    @State private var spotlight = TVHomeSpotlight()
    @State private var launchFocusPending = true
    @State private var preparation: Task<Void, Never>?
    @Environment(\.accessibilityReduceMotion) private var reduceMotion
    #else
    @State private var scroll = SpotlightScroll()
    #endif
    private var homeMode: PlayerMode { mode ?? .watch }
    var body: some View {
        ScrollView {
            ResourceView(identity: "\(session.profileKey ?? ""):\(homeMode.rawValue)", refreshID: session.contentRevision.uuidString,
                         loadingLayout: tvHomeLoadingLayout, allowsPullToRefresh: false, load: { policy in
                guard let client = session.client else { throw ClientError.http(401) }
                return try await client.home(mode: homeMode, policy: policy)
            }) { home in
                let selection = HomeSelection(continueWatching: home.continueWatching, recent: home.recent, mode: homeMode)
                LazyVStack(alignment: .leading, spacing: 32) {
                    #if os(tvOS)
                    if homeMode == .watch {
                        ScrollViewReader { proxy in
                            VStack(alignment: .leading, spacing: 32) {
                                if selection.featured != nil {
                                    TVHomeSpotlightHeader(spotlight: spotlight, fallback: selection.featured).id(Self.topID)
                                }
                                if !selection.tvWatchingRail.isEmpty {
                                    MediaShelf(title: "Continue watching", items: selection.tvWatchingRail, landscape: true,
                                               resumesPlayback: true, onQuickPlay: { session.open($0) { quickPlay = $0 } },
                                               onFocus: { showWatching($0, proxy: proxy) },
                                               takesFocus: takesLaunchFocus, onTookFocus: { launchFocusPending = false })
                                }
                            }
                        }
                    } else if !selection.featuredAndContinuation.isEmpty {
                        ResumeRows(items: selection.featuredAndContinuation, title: "Listening", showsAll: false)
                    }
                    TVHomeBrowse(selectTab: selectTab)
                    #else
                    if let featured = selection.featured, !featured.spotlightArtwork.isEmpty {
                        CinemaSpotlight(item: featured, artwork: featured.spotlightArtwork, eyebrow: featuredHeading(selection),
                                        subtitle: homeSubtitle(for: featured), scroll: scroll) {
                            MediaDestinationLink(destination: featured.playingDestination, item: featured) {
                                Label(featured.playLabel, systemImage: featured.kind == .book ? "book.fill" : "play.fill")
                                    .padding(.horizontal, 6)
                            }
                            .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                            NavigationLink("Details", value: featured.destination).buttonStyle(SpotlightSecondaryButtonStyle())
                            NavigationLink(value: ScreenDestination.library(.list)) {
                                Image(systemName: "bookmark").accessibilityLabel("My List")
                            }
                            .buttonStyle(SpotlightSecondaryButtonStyle()).buttonBorderShape(.circle)
                        }
                    } else {
                        VStack(alignment: .leading, spacing: 12) {
                            HStack(spacing: 28) {
                                Text(featuredHeading(selection))
                                    .font(.title2.bold()).frame(minHeight: 44).accessibilityAddTraits(.isHeader)
                                NavigationLink("My List", value: ScreenDestination.library(.list))
                                    .font(.callout).frame(minHeight: 44)
                                    .foregroundStyle(KinoTheme.muted)
                            }
                            .frame(maxWidth: .infinity, alignment: .leading)
                            if let featured = selection.featured {
                                CinemaHero(item: featured, subtitle: homeSubtitle(for: featured), showsPlot: false) {
                                    MediaDestinationLink(destination: featured.playingDestination, item: featured) {
                                        Label(featured.playLabel, systemImage: featured.kind == .book ? "book.fill" : "play.fill")
                                            .frame(maxWidth: .infinity)
                                    }
                                    .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                                    NavigationLink("Details", value: featured.destination).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
                                }
                            }
                        }
                    }
                    if !selection.continuation.isEmpty {
                        if homeMode == .listen {
                            ResumeRows(items: selection.continuation, title: "Continue listening", showsAll: false)
                        } else {
                            MediaShelf(title: "Continue watching", items: selection.watchShelf,
                                       landscape: true, resumesPlayback: true)
                        }
                    }
                    #endif
                    if homeMode == .watch {
                        let lead = selection.tvLeadRecentShelf
                        recentShelf("Recently added movies", items: selection.recent(for: .video), leads: lead == 0)
                        recentShelf("Recently added TV shows", items: selection.recent(for: .show), opensShows: true, leads: lead == 1)
                        recentShelf("Unwatched TV shows", items: selection.unwatchedShows, opensShows: true, leads: lead == 2)
                        recentShelf("Unwatched movies", items: selection.unwatchedMovies, leads: lead == 3)
                        ForEach(selection.movieGenres) { genre in
                            recentShelf(LocalizedStringKey(genre.name), items: genre.items)
                        }
                    } else {
                        recentShelf("Recently added music", items: selection.recent(for: .music))
                        recentShelf("Recently added audiobooks", items: selection.recent(for: .audiobook))
                    }
                    if selection.featured == nil {
                        FeaturePlaceholder(title: homeMode == .listen ? "Nothing to listen to yet" : "Your library is ready",
                                           symbol: homeMode == .listen ? "headphones" : "play.rectangle",
                                           message: homeMode == .listen ? "Add music or audiobooks to your Server to see them here." : "Media added to your Server will appear here.")
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
        #if os(tvOS)
        .focusSection()
        .mask {
            VStack(spacing: 0) {
                LinearGradient(colors: [.clear, .black], startPoint: .top, endPoint: .bottom).frame(height: 32)
                Color.black
            }
            .ignoresSafeArea(edges: [.bottom, .horizontal])
        }
        .onExitCommand { focusTopBar?() }
        .navigationTitle("")
        .navigationDestination(item: $quickPlay) { DestinationScreen(destination: $0) }
        #else
        .spotlightScroll(scroll)
        .cinemaBackground().tabBarCollapses()
        .navigationTitle("Kinosail")
        #endif
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar {
            #if os(iOS)
            if let mode, let changeMode, !session.usesDeviceLibrary {
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

    private func featuredHeading(_ selection: HomeSelection) -> String {
        selection.featuredIsContinuing ? (homeMode == .listen ? String(localized: "Listening") : String(localized: "Watching")) : String(localized: "For you")
    }

    private func homeSubtitle(for item: MediaItem) -> String {
        #if os(tvOS)
        item.subtitleWithoutYear
        #else
        item.kind == .video || item.kind == .show ? item.subtitleWithoutYear : item.subtitle
        #endif
    }

    #if os(tvOS)
    private static let topID = "home.top"
    private var takesLaunchFocus: Bool { launchFocus && launchFocusPending }

    private func showWatching(_ item: MediaItem, proxy: ScrollViewProxy) {
        spotlight.show(item)
        withAnimation(reduceMotion ? nil : .easeOut(duration: 0.25)) { proxy.scrollTo(Self.topID, anchor: .top) }
        preparation?.cancel()
        preparation = Task {
            try? await Task.sleep(for: .milliseconds(400))
            guard !Task.isCancelled, let client = session.client else { return }
            try? await session.player.prepare(item, client: client)
        }
    }
    #endif

    private var tvHomeLoadingLayout: LoadingLayout {
        #if os(tvOS)
        homeMode == .listen ? .tvHomeAudio : .tvHome
        #else
        homeMode == .listen ? .homeAudio : .home
        #endif
    }

    @ViewBuilder private func recentShelf(_ title: LocalizedStringKey, items: [MediaItem], opensShows: Bool = false, leads: Bool = false) -> some View {
        if !items.isEmpty {
            #if os(tvOS)
            MediaShelf(title: title, items: items, onQuickPlay: { session.open($0) { quickPlay = $0 } }, opensShows: opensShows,
                       takesFocus: leads && takesLaunchFocus, onTookFocus: { launchFocusPending = false })
            #else
            MediaShelf(title: title, items: items, opensShows: opensShows)
            #endif
        }
    }
}

struct HomeSelection {
    struct MovieGenre: Identifiable {
        let name: String
        let items: [MediaItem]
        var id: String { name }
    }

    let featured: MediaItem?
    let featuredIsContinuing: Bool
    let continuation: [MediaItem]
    let recent: [MediaItem]
    var watchShelf: [MediaItem] { Array(continuation.prefix(15)) }
    var featuredAndContinuation: [MediaItem] { (featuredIsContinuing ? [featured].compactMap { $0 } : []) + continuation }
    var tvWatchingRail: [MediaItem] { Array(featuredAndContinuation.prefix(15)) }

    init(continueWatching: [MediaItem], recent: [MediaItem], mode: PlayerMode? = nil) {
        let watching = continueWatching.filter { mode?.includes($0) ?? true }
        let added = recent.filter { mode?.includes($0) ?? true }
        featured = watching.first ?? added.first
        featuredIsContinuing = !watching.isEmpty
        continuation = Array(watching.dropFirst())
        self.recent = added
    }

    func recent(for kind: MediaKind) -> [MediaItem] {
        recent.filter { item in
            switch kind {
            case .video: item.kind == .video && item.showID.isEmpty
            case .show: item.kind == .show || item.kind == .video && !item.showID.isEmpty
            default: item.kind == kind
            }
        }
    }

    var unwatchedMovies: [MediaItem] { recent(for: .video).filter { !$0.progress.watched } }
    var tvLeadRecentShelf: Int? {
        guard tvWatchingRail.isEmpty else { return nil }
        return [recent(for: .video), recent(for: .show), unwatchedShows, unwatchedMovies].firstIndex { !$0.isEmpty }
    }
    var unwatchedShows: [MediaItem] { recent(for: .show).filter { !$0.progress.watched } }
    var movieGenres: [MovieGenre] {
        var grouped: [String: [MediaItem]] = [:]
        for movie in recent(for: .video) {
            let names = Set(movie.genres.components(separatedBy: " · ").map { $0.trimmingCharacters(in: .whitespaces) })
            for name in names where !name.isEmpty { grouped[name, default: []].append(movie) }
        }
        return Array(grouped.map { MovieGenre(name: $0.key, items: $0.value) }
            .sorted { $0.items.count == $1.items.count ? $0.name < $1.name : $0.items.count > $1.items.count }
            .prefix(4))
    }
}
