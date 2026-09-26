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
    private var homeMode: PlayerMode { mode ?? .watch }
    var body: some View {
        ScrollView {
            HStack(spacing: 28) {
                Text("For you").font(.title2.bold()).frame(minHeight: 44)
                    .accessibilityAddTraits(.isHeader)
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
            .padding(.horizontal, KinoTheme.contentPadding)

            // Refresh belongs to a vertical scroll container, not the nested media shelf.
            ResourceView(identity: "\(session.profileKey ?? ""):\(homeMode.rawValue)", refreshID: session.contentRevision.uuidString, loadingLayout: homeMode == .listen ? .homeAudio : .home, allowsPullToRefresh: false, load: { policy in
                guard let client = session.client else { throw ClientError.http(401) }
                return try await client.home(mode: homeMode, policy: policy)
            }) { home in
                let selection = HomeSelection(continueWatching: home.continueWatching, recent: home.recent, mode: homeMode)
                VStack(alignment: .leading, spacing: 32) {
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
                    if !selection.continuation.isEmpty {
                        ResumeRows(items: selection.continuation, title: homeMode == .listen ? "Continue listening" : "Continue watching",
                                   showsAll: homeMode != .listen)
                    }
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
        item.subtitle
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
    let continuation: [MediaItem]
    let recent: [MediaItem]

    init(continueWatching: [MediaItem], recent: [MediaItem], mode: PlayerMode? = nil) {
        let watching = continueWatching.filter { mode?.includes($0) ?? true }
        let added = recent.filter { mode?.includes($0) ?? true }
        featured = watching.first ?? added.first
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
