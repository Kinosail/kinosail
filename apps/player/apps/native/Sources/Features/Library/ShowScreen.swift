import SwiftUI

struct ShowScreen: View {
    let showID: String
    @Environment(AppSession.self) private var session
    #if os(tvOS)
    @State private var selectedSeason: Int?
    @Namespace private var showFocus
    @State private var quickPlay: ScreenDestination?
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
                    if let next = ShowSeasonSelection.featuredEpisode(in: episodes) {
                        #if os(tvOS)
                        let groups = ShowSeasonSelection.groups(episodes)
                        let season = ShowSeasonSelection.resolve(selectedSeason, among: groups.map(\.number), defaultingTo: next.season) ?? 0
                        VStack(alignment: .leading, spacing: 16) {
                            Text(next.show.isEmpty ? "Episodes" : next.show)
                                .font(.system(.largeTitle, design: .rounded).weight(.bold))
                                .fixedSize(horizontal: false, vertical: true)
                                .accessibilityAddTraits(.isHeader)
                            if !next.title.isEmpty {
                                Text(next.title).font(.title3).foregroundStyle(.secondary)
                            }
                            NavigationLink(value: ScreenDestination.playback(next.id)) {
                                Label("\(next.playLabel) · S\(next.season) E\(next.episode)", systemImage: "play.fill")
                                    .frame(minWidth: 240).padding(.vertical, 12)
                                    .foregroundStyle(KinoTheme.tvOSPrimaryInk)
                                    .background(KinoTheme.tvOSPrimaryFill, in: Capsule())
                            }
                            .buttonStyle(.card)
                            .tvOSDefaultPlayFocus(in: showFocus, id: "show.next-play.\(next.id)")
                            .controlSize(.large)
                        }
                        .foregroundStyle(KinoTheme.text)
                        .focusSection()
                        #else
                        CinemaHero(item: next, title: next.show.isEmpty ? "Episodes" : next.show,
                                   subtitle: next.title, showsPlot: false) {
                            NavigationLink(value: ScreenDestination.playback(next.id)) {
                                Label("\(next.playLabel) · S\(next.season) E\(next.episode)", systemImage: "play.fill")
                            }.buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                        }
                        #endif
                        #if os(iOS)
                        IOSShowEpisodes(episodes: episodes, nextID: next.id, jumpTo: jumpTo)
                            .id(showID)
                        #endif
                        #if os(tvOS)
                        HStack(alignment: .top, spacing: 32) {
                            VStack(alignment: .leading, spacing: 16) {
                                Text("Seasons").font(.title2.bold()).accessibilityAddTraits(.isHeader)
                                ForEach(groups) { group in
                                    Button { selectedSeason = group.number } label: {
                                        HStack {
                                            Text(group.title)
                                            Spacer()
                                            if group.number == season { Image(systemName: "checkmark").accessibilityHidden(true) }
                                        }
                                        .font(.headline)
                                        .foregroundStyle(group.number == season ? KinoTheme.signal : KinoTheme.text)
                                        .padding(16)
                                        .frame(maxWidth: .infinity, alignment: .leading)
                                        .background(KinoTheme.surface, in: RoundedRectangle(cornerRadius: 12))
                                    }
                                    .buttonStyle(.card)
                                    .accessibilityAddTraits(group.number == season ? .isSelected : [])
                                    .accessibilityIdentifier("show.season-\(group.number)")
                                }
                            }
                            .frame(width: 220)
                            .focusSection()
                            MediaShelf(title: "Episodes", items: episodes.filter { $0.season == season }, landscape: true,
                                       onQuickPlay: { quickPlay = $0 })
                                .id(season)
                                .frame(maxWidth: .infinity, alignment: .leading)
                        }
                        CastShelf(people: show.cast)
                        #endif
                    } else { ContentUnavailableView("No episodes", systemImage: "tv", description: Text("This show has no available episodes.")) }
                    #if os(iOS)
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
