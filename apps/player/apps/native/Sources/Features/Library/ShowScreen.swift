import SwiftUI

struct ShowScreen: View {
    let showID: String
    @Environment(AppSession.self) private var session
    #if os(tvOS)
    @State private var selectedSeason: Int?
    @Namespace private var showFocus
    @State private var quickPlay: ScreenDestination?
    @Environment(\.dynamicTypeSize) private var dynamicType
    @ScaledMetric(relativeTo: .body) private var playHeight = 44
    #else
    @State private var scroll = SpotlightScroll()
    #endif

    var body: some View {
        #if os(iOS)
        ScrollViewReader { scroll in
            page { scroll.scrollTo("season-\($0)", anchor: .top) }
        }
        #else
        page { _ in }
        #endif
    }

    private func page(jumpTo: @escaping (Int) -> Void) -> some View {
        ScrollView {
            ResourceView(identity: "\(session.profileKey ?? ""):\(showID)", refreshID: session.contentRevision.uuidString, loadingLayout: .show, load: { policy in
                guard let client = session.client else { throw ClientError.unavailable }
                return try await client.show(id: showID, policy: policy)
            }) { show in
                let episodes = show.episodes
                VStack(alignment: .leading, spacing: 28) {
                    #if os(tvOS)
                    let next = ShowSeasonSelection.featuredEpisode(in: episodes)
                    tvHero(show: show, next: next)
                    if let next {
                        let groups = ShowSeasonSelection.groups(episodes)
                        let season = ShowSeasonSelection.resolve(selectedSeason, among: groups.map(\.number), defaultingTo: next.season) ?? next.season
                        tvSeasons(groups, selected: season)
                        MediaShelf(title: "Episodes", items: episodes.filter { $0.season == season },
                                   landscape: true, onQuickPlay: { session.open($0) { quickPlay = $0 } }, playsOnSelect: true)
                            .id(season)
                    } else {
                        ContentUnavailableView("No episodes", systemImage: "tv",
                                               description: Text("This show has no available episodes."))
                    }
                    CastShelf(people: show.cast)
                    #else
                    if let next = ShowSeasonSelection.featuredEpisode(in: episodes) {
                        CinemaSpotlight(item: next, artwork: show.backdrop.isEmpty ? next.poster : show.backdrop,
                                        eyebrow: "\(String(localized: "Up next")) · S\(next.season) E\(next.episode)",
                                        title: show.title.isEmpty ? next.show : show.title,
                                        subtitle: [show.year, show.genres].filter { !$0.isEmpty }.joined(separator: " · "),
                                        scroll: scroll) {
                            MediaDestinationLink(destination: .playback(next.id), item: next) {
                                Label(next.playLabel, systemImage: "play.fill").padding(.horizontal, 6)
                            }.buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                            NavigationLink("Details", value: ScreenDestination.detail(next.id)).buttonStyle(SpotlightSecondaryButtonStyle())
                        }
                        if !show.plot.isEmpty {
                            Text(show.plot).font(.body).fixedSize(horizontal: false, vertical: true).frame(maxWidth: 900, alignment: .leading)
                        }
                        IOSShowEpisodes(episodes: episodes, nextID: next.id, jumpTo: jumpTo)
                            .id(showID)
                    } else { ContentUnavailableView("No episodes", systemImage: "tv", description: Text("This show has no available episodes.")) }
                    IOSShowCast(people: show.cast)
                    #endif
                }
            }
            .padding(KinoTheme.contentPadding)
        }
        #if os(tvOS)
        .tvMovieHeroViewport()
        #else
        .spotlightScroll(scroll)
        #endif
        .cinemaBackground().tabBarCollapses()
        #if os(tvOS)
        .focusScope(showFocus)
        .navigationDestination(item: $quickPlay) { DestinationScreen(destination: $0) }
        #endif
        #if os(tvOS)
        .navigationTitle("")
        #else
        .navigationTitle("")
        .navigationBarTitleDisplayMode(.inline)
        #endif
        #if os(tvOS)
        .onChange(of: showID) { _, _ in selectedSeason = nil }
        #endif
    }

    #if os(tvOS)
    private func tvHero(show: ShowDetail, next: MediaItem?) -> some View {
        let metadata = [show.year, show.genres].filter { !$0.isEmpty }.joined(separator: " · ")
        return TVMovieHeroLayout { VStack(alignment: .leading, spacing: 14) {
            Text(show.title)
                .font(.system(.largeTitle, design: .rounded).weight(.bold))
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityAddTraits(.isHeader)
            if !metadata.isEmpty { Text(metadata).font(.callout).foregroundStyle(KinoTheme.muted) }
            if !show.plot.isEmpty {
                Text(show.plot).font(.body).lineLimit(dynamicType.isAccessibilitySize ? nil : 4)
                    .fixedSize(horizontal: false, vertical: true)
            }
            if let next {
                Text(ShowSeasonSelection.episodeSummary(next))
                    .font(.headline).foregroundStyle(KinoTheme.muted)
                if next.progress.seconds > 0 && !next.progress.watched { WatchPosition(item: next) }
                MediaDestinationLink(destination: .playback(next.id), item: next) {
                    Label(next.playLabel, systemImage: "play.fill")
                        .frame(minWidth: 240, minHeight: playHeight)
                        .foregroundStyle(KinoTheme.tvOSPrimaryInk)
                }
                .tvCapsule(prominent: true)
                .tvOSDefaultPlayFocus(in: showFocus, id: "show.next-play.\(next.id)")
            }
        } }
        .tvMovieHeroArtwork(show.backdrop)
        .foregroundStyle(KinoTheme.text)
        .tvMovieHeroBleed()
        .focusSection()
        .task(id: "\(session.profileKey ?? ""):\(next?.id ?? "")") {
            guard let next, let client = session.client else { return }
            try? await session.player.prepare(next, client: client)
        }
    }

    private func tvSeasons(_ groups: [ShowSeasonSelection.Group], selected: Int) -> some View {
        ScrollView(.horizontal) {
            HStack(spacing: 16) {
                ForEach(groups) { group in
                    let isSelected = group.number == selected
                    Button(group.title) { selectedSeason = group.number }
                        .tvCapsule(prominent: isSelected)
                        .accessibilityAddTraits(isSelected ? .isSelected : [])
                        .accessibilityIdentifier("show.season-\(group.number)")
                }
            }
            .padding(.horizontal, 24).padding(.vertical, 18)
        }
        .scrollIndicators(.hidden)
        .scrollClipDisabled()
        .padding(.horizontal, -24)
        .focusSection()
    }
    #endif
}

enum ShowSeasonSelection {
    struct Group: Identifiable {
        let number: Int
        let episodes: [MediaItem]
        var id: Int { number }
        var title: String { number == 0 ? "Specials" : "Season \(number)" }
        var watchedCount: Int { episodes.filter(\.progress.watched).count }
    }

    static func groups(_ episodes: [MediaItem]) -> [Group] {
        let grouped = Dictionary(grouping: episodes, by: \.season)
        return grouped.keys.sorted().map { Group(number: $0, episodes: grouped[$0] ?? []) }
    }

    static func episodeTitle(_ item: MediaItem) -> String {
        let prefix = String(format: "S%02dE%02d · ", item.season, item.episode)
        return item.title.hasPrefix(prefix) ? String(item.title.dropFirst(prefix.count)) : item.title
    }

    static func episodeSummary(_ item: MediaItem) -> String {
        (["S\(item.season)", "E\(item.episode)", episodeTitle(item), item.rating].filter { !$0.isEmpty }).joined(separator: " · ")
    }

    static func featuredEpisode(in episodes: [MediaItem]) -> MediaItem? {
        episodes.first(where: { !$0.progress.watched }) ?? episodes.first
    }

    static func resolve(_ selected: Int?, among seasons: [Int], defaultingTo featured: Int?) -> Int? {
        selected.flatMap { seasons.contains($0) ? $0 : nil }
            ?? featured.flatMap { seasons.contains($0) ? $0 : nil }
            ?? seasons.first
    }
}
