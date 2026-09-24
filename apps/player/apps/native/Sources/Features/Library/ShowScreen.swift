import SwiftUI

struct ShowScreen: View {
    let showID: String
    @Environment(AppSession.self) private var session
    @State private var selectedSeason: Int?
    #if os(tvOS)
    @Namespace private var showFocus
    #endif
    #if os(iOS)
    @State private var downloading = false
    @State private var message: String?
    #endif

    var body: some View {
        ScrollView {
            ResourceView(identity: "\(session.profileKey ?? ""):\(showID)", refreshID: session.contentRevision.uuidString, load: { policy in
                guard let client = session.client else { throw ClientError.unavailable }
                return try await client.show(id: showID, policy: policy)
            }) { show in
                let episodes = show.episodes
                VStack(alignment: .leading, spacing: 28) {
                    if let next = ShowSeasonSelection.featuredEpisode(in: episodes) {
                        let seasons = Array(Set(episodes.map(\.season))).sorted()
                        let season = ShowSeasonSelection.resolve(selectedSeason, among: seasons, defaultingTo: next.season) ?? 0
                        #if os(tvOS)
                        VStack(alignment: .leading, spacing: 16) {
                            Text(next.show.isEmpty ? "Episodes" : next.show)
                                .font(.system(.largeTitle, design: .rounded).weight(.bold))
                                .fixedSize(horizontal: false, vertical: true)
                                .accessibilityAddTraits(.isHeader)
                            if !next.title.isEmpty {
                                Text(next.title).font(.title3).foregroundStyle(.secondary)
                            }
                            HStack(spacing: 16) {
                                NavigationLink(value: ScreenDestination.playback(next.id)) {
                                    Label("\(next.playLabel) · S\(next.season) E\(next.episode)", systemImage: "play.fill")
                                }
                                .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                                .tvOSDefaultPlayFocus(in: showFocus, id: "show.next-play.\(next.id)")
                                Picker("Season", selection: Binding(get: { season }, set: { selectedSeason = $0 })) {
                                    ForEach(seasons, id: \.self) { Text($0 == 0 ? "Specials" : "Season \($0)").tag($0) }
                                }
                                .pickerStyle(.menu)
                                .accessibilityIdentifier("show.season-picker")
                            }
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
                        Picker("Season", selection: Binding(get: { season }, set: { selectedSeason = $0 })) {
                            ForEach(seasons, id: \.self) { Text($0 == 0 ? "Specials" : "Season \($0)").tag($0) }
                        }.frame(maxWidth: 420).accessibilityIdentifier("show.season-picker")
                        if session.viewer?.downloads == true {
                            Button(downloading ? "Adding episodes…" : "Download season", systemImage: "arrow.down.circle") {
                                guard let client = session.client, !downloading else { return }
                                downloading = true
                                let chosen = episodes.filter { $0.season == season }
                                Task {
                                    defer { downloading = false }
                                    do { try await session.downloads.enqueueEpisodes(chosen, quality: .compatible, client: client); message = "Episodes added to Downloads." }
                                    catch { message = AppSession.message(error) }
                                }
                            }.disabled(downloading)
                            Text("Compatible video with all audio tracks. Choose an individual episode to include subtitles.").font(.caption).foregroundStyle(.secondary)
                            if let message { Text(message).font(.callout).foregroundStyle(.secondary) }
                        }
                        #endif
                        #if os(tvOS)
                        MediaGrid(landscape: true, items: episodes.filter { $0.season == season })
                        CastShelf(people: show.cast)
                        #else
                        CastShelf(people: show.cast)
                        MediaGrid(landscape: true, items: episodes.filter { $0.season == season })
                        #endif
                    } else { ContentUnavailableView("No episodes", systemImage: "tv", description: Text("This show has no available episodes.")) }
                }
            }
            .padding(KinoTheme.contentPadding)
        }
        .cinemaBackground()
        #if os(tvOS)
        .focusScope(showFocus)
        #endif
        #if os(tvOS)
        .navigationTitle("")
        #else
        .navigationTitle("Seasons & episodes")
        #endif
        .onChange(of: showID) { _, _ in selectedSeason = nil }
    }
}

enum ShowSeasonSelection {
    static func featuredEpisode(in episodes: [MediaItem]) -> MediaItem? {
        episodes.first(where: { !$0.progress.watched }) ?? episodes.first
    }

    static func resolve(_ selected: Int?, among seasons: [Int], defaultingTo featured: Int? = nil) -> Int? {
        selected.flatMap { seasons.contains($0) ? $0 : nil }
            ?? featured.flatMap { seasons.contains($0) ? $0 : nil }
            ?? seasons.first
    }
}
