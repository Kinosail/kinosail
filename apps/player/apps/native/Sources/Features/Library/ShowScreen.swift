import SwiftUI

struct ShowScreen: View {
    let showID: String
    @Environment(AppSession.self) private var session
    #if os(tvOS)
    @State private var selectedSeason: Int?
    @Namespace private var showFocus
    @State private var quickPlay: ScreenDestination?
    @Environment(\.colorSchemeContrast) private var contrast
    @Environment(\.accessibilityReduceTransparency) private var reduceTransparency
    @Environment(\.dynamicTypeSize) private var dynamicType
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
                                   landscape: true, onQuickPlay: { quickPlay = $0 })
                            .id(season)
                    } else {
                        ContentUnavailableView("No episodes", systemImage: "tv",
                                               description: Text("This show has no available episodes."))
                    }
                    CastShelf(people: show.cast)
                    #else
                    if let next = ShowSeasonSelection.featuredEpisode(in: episodes) {
                        CinemaHero(item: next, title: next.show.isEmpty ? "Episodes" : next.show,
                                   subtitle: next.title, showsPlot: false) {
                            NavigationLink(value: ScreenDestination.playback(next.id)) {
                                Label("\(next.playLabel) · S\(next.season) E\(next.episode)", systemImage: "play.fill")
                            }.buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
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
        .cinemaBackground()
        #if os(tvOS)
        .focusScope(showFocus)
        .navigationDestination(item: $quickPlay) { DestinationScreen(destination: $0) }
        #endif
        #if os(tvOS)
        .navigationTitle("")
        #else
        .navigationTitle("Seasons & episodes")
        #endif
        #if os(tvOS)
        .onChange(of: showID) { _, _ in selectedSeason = nil }
        #endif
    }

    #if os(tvOS)
    private func tvHero(show: ShowDetail, next: MediaItem?) -> some View {
        let metadata = [show.year, show.genres].filter { !$0.isEmpty }.joined(separator: " · ")
        return VStack(alignment: .leading, spacing: 14) {
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
                Text("S\(next.season) · E\(next.episode) · \(ShowSeasonSelection.episodeTitle(next))")
                    .font(.headline).foregroundStyle(KinoTheme.muted)
                NavigationLink(value: ScreenDestination.playback(next.id)) {
                    Label(next.playLabel, systemImage: "play.fill")
                        .frame(minWidth: 240).padding(.vertical, 12)
                        .foregroundStyle(KinoTheme.tvOSPrimaryInk)
                        .background(KinoTheme.tvOSPrimaryFill, in: Capsule())
                }
                .buttonStyle(.card)
                .tvOSDefaultPlayFocus(in: showFocus, id: "show.next-play.\(next.id)")
            }
        }
        .frame(maxWidth: 740, alignment: .leading)
        .padding(40)
        .frame(maxWidth: .infinity, minHeight: 480, alignment: .bottomLeading)
        .background {
            ZStack {
                KinoTheme.surface
                if !show.backdrop.isEmpty && contrast != .increased && !reduceTransparency {
                    GeometryReader { geometry in
                        Artwork(path: show.backdrop, ratio: 16 / 9, dimension: 1920,
                                fillsFrame: true, isBackdrop: true, canvasSize: geometry.size)
                    }
                    LinearGradient(colors: [.black.opacity(0.88), .black.opacity(0.38), .clear],
                                   startPoint: .leading, endPoint: .trailing)
                    LinearGradient(colors: [.clear, .black.opacity(0.75)], startPoint: .top, endPoint: .bottom)
                }
            }
        }
        .clipShape(.rect(cornerRadius: 12))
        .foregroundStyle(KinoTheme.text)
        .focusSection()
    }

    private func tvSeasons(_ groups: [ShowSeasonSelection.Group], selected: Int) -> some View {
        ScrollView(.horizontal) {
            HStack(spacing: 16) {
                ForEach(groups) { group in
                    let isSelected = group.number == selected
                    Button { selectedSeason = group.number } label: {
                        Text(group.title)
                            .font(.headline)
                            .foregroundStyle(isSelected ? KinoTheme.signalInk : KinoTheme.text)
                            .padding(.horizontal, 20).padding(.vertical, 12)
                            .background(isSelected ? KinoTheme.signal : KinoTheme.raised, in: Capsule())
                    }
                    .buttonStyle(.card)
                    .accessibilityAddTraits(isSelected ? .isSelected : [])
                    .accessibilityIdentifier("show.season-\(group.number)")
                }
            }
            .padding(.horizontal, 24).padding(.vertical, 18)
        }
        .scrollIndicators(.hidden)
        .scrollClipDisabled()
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

    static func featuredEpisode(in episodes: [MediaItem]) -> MediaItem? {
        episodes.first(where: { !$0.progress.watched }) ?? episodes.first
    }

    static func resolve(_ selected: Int?, among seasons: [Int], defaultingTo featured: Int?) -> Int? {
        selected.flatMap { seasons.contains($0) ? $0 : nil }
            ?? featured.flatMap { seasons.contains($0) ? $0 : nil }
            ?? seasons.first
    }
}
