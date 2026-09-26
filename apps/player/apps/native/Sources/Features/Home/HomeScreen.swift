import SwiftUI

struct HomeScreen: View {
    var showsSearch = true
    var mode: PlayerMode?
    var changeMode: ((PlayerMode) -> Void)?
    var selectTab: (PlayerTab) -> Void
    @Environment(AppSession.self) private var session
    #if os(tvOS)
    @Namespace private var homeFocus
    @State private var quickPlay: ScreenDestination?
    #endif
    private var homeMode: PlayerMode { mode ?? .watch }
    var body: some View {
        ScrollView {
            // Refresh belongs to a vertical scroll container, not the nested media shelf.
            ResourceView(identity: "\(session.profileKey ?? ""):\(homeMode.rawValue)", refreshID: session.contentRevision.uuidString,
                         loadingLayout: tvHomeLoadingLayout, allowsPullToRefresh: false, load: { policy in
                guard let client = session.client else { throw ClientError.http(401) }
                return try await client.home(mode: homeMode, policy: policy)
            }) { home in
                let selection = HomeSelection(continueWatching: home.continueWatching, recent: home.recent, mode: homeMode)
                VStack(alignment: .leading, spacing: 32) {
                    #if os(tvOS)
                    if homeMode == .watch, !selection.tvWatchingRail.isEmpty {
                        MediaShelf(title: "Watching", items: selection.tvWatchingRail, landscape: true,
                                   resumesPlayback: true, onQuickPlay: { quickPlay = $0 })
                    } else if homeMode == .listen, !selection.featuredAndContinuation.isEmpty {
                        ResumeRows(items: selection.featuredAndContinuation, title: "Listening", showsAll: false)
                    }
                    TVHomeBrowse(mode: homeMode, selectTab: selectTab, changeMode: changeMode)
                    #else
                    VStack(alignment: .leading, spacing: 12) {
                        HStack(spacing: 28) {
                            Text(selection.featuredIsContinuing ? (homeMode == .listen ? "Listening" : "Watching") : "For you")
                                .font(.title2.bold()).frame(minHeight: 44).accessibilityAddTraits(.isHeader)
                            NavigationLink("My List", value: ScreenDestination.library(.list))
                                .font(.callout).frame(minHeight: 44)
                                .foregroundStyle(KinoTheme.muted)
                        }
                        .frame(maxWidth: .infinity, alignment: .leading)
                        if let featured = selection.featured {
                            CinemaHero(item: featured, subtitle: homeSubtitle(for: featured), showsPlot: false) {
                                NavigationLink(value: featured.playingDestination) {
                                    Label(featured.playLabel, systemImage: featured.kind == .book ? "book.fill" : "play.fill")
                                        .frame(maxWidth: .infinity)
                                }
                                .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                                NavigationLink("Details", value: featured.destination).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
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
                        recentShelf("Recently added movies", items: selection.recent(for: .video))
                        recentShelf("Recently added TV shows", items: selection.recent(for: .show), opensShows: true)
                        recentShelf("Unwatched TV shows", items: selection.unwatchedShows, opensShows: true)
                        recentShelf("Unwatched movies", items: selection.unwatchedMovies)
                        if !selection.movieGenres.isEmpty {
                            VStack(alignment: .leading, spacing: 18) {
                                Text("Movie genres").font(.title2.bold()).accessibilityAddTraits(.isHeader)
                                ForEach(selection.movieGenres) { genre in
                                    recentShelf(genre.name, items: genre.items)
                                }
                            }
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
        item.kind == .video || item.kind == .show ? item.subtitleWithoutYear : item.subtitle
        #endif
    }

    private var tvHomeLoadingLayout: LoadingLayout {
        #if os(tvOS)
        homeMode == .listen ? .tvHomeAudio : .tvHome
        #else
        homeMode == .listen ? .homeAudio : .home
        #endif
    }

    @ViewBuilder private func recentShelf(_ title: String, items: [MediaItem], opensShows: Bool = false) -> some View {
        if !items.isEmpty {
            #if os(tvOS)
            MediaShelf(title: title, items: items, onQuickPlay: { quickPlay = $0 }, opensShows: opensShows)
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

#if os(tvOS)
private struct TVHomeBrowse: View {
    let mode: PlayerMode
    let selectTab: (PlayerTab) -> Void
    let changeMode: ((PlayerMode) -> Void)?
    private let tabs: [PlayerTab] = [.movies, .shows, .music, .audiobooks, .photos, .collections]

    var body: some View {
        VStack(alignment: .leading, spacing: 16) {
            Text("Browse").font(.title2.bold()).accessibilityAddTraits(.isHeader)
            ScrollView(.horizontal) {
                LazyHStack(alignment: .top, spacing: 18) {
                    ForEach(tabs) { tab in
                        tile(title: tab.title, symbol: tab.symbol, color: fill(for: tab)) { selectTab(tab) }
                    }
                    if let changeMode {
                        tile(title: "\(mode.other.title) Home", symbol: mode.other == .listen ? "headphones" : "tv",
                             color: KinoTheme.raised) { changeMode(mode.other) }
                    }
                }
                .padding(.horizontal, 24)
                .padding(.vertical, 24)
                .scrollTargetLayout()
            }
            .scrollIndicators(.hidden)
            .scrollTargetBehavior(.viewAligned)
            .scrollClipDisabled()
        }
        .focusSection()
    }

    private func tile(title: String, symbol: String, color: Color, action: @escaping () -> Void) -> some View {
        VStack(spacing: 10) {
            Button(action: action) {
                RoundedRectangle(cornerRadius: 14)
                    .fill(color)
                    .overlay {
                        ZStack(alignment: .topTrailing) {
                            Image(systemName: symbol).font(.system(size: 66, weight: .light)).foregroundStyle(.white)
                                .frame(width: 108, height: 92)
                            Image("KinosailMark").resizable().scaledToFit().frame(width: 24, height: 31)
                                .padding(5).background(KinoTheme.background.opacity(0.78), in: RoundedRectangle(cornerRadius: 9))
                                .offset(x: 10, y: -3)
                        }
                    }
                    .frame(width: 320, height: 150)
            }
            .buttonStyle(.card)
            .accessibilityLabel(title)
            Text(title).font(.headline).foregroundStyle(KinoTheme.text).accessibilityHidden(true)
        }
        .frame(width: 320)
    }

    private func fill(for tab: PlayerTab) -> Color {
        switch tab {
        case .library: Color(red: 0.92, green: 0.32, blue: 0.18)
        case .movies: Color(red: 0.07, green: 0.42, blue: 0.90)
        case .shows: Color(red: 0.72, green: 0.16, blue: 0.66)
        case .music: Color(red: 0.11, green: 0.64, blue: 0.30)
        case .audiobooks: Color(red: 0.42, green: 0.28, blue: 0.80)
        case .photos: Color(red: 0.06, green: 0.55, blue: 0.67)
        default: Color(red: 0.47, green: 0.38, blue: 0.68)
        }
    }
}
#endif
