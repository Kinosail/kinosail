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
                    if let first = episodes.first {
                        if let next = episodes.first(where: { !$0.progress.watched }) ?? episodes.first {
                            CinemaHero(item: first, title: first.show.isEmpty ? "Episodes" : first.show, showsPlot: false) {
                                NavigationLink(value: ScreenDestination.playback(next.id)) {
                                    Label("\(next.playLabel) · S\(next.season) E\(next.episode)", systemImage: "play.fill")
                                }.buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                                #if os(tvOS)
                                .tvOSDefaultPlayFocus(in: showFocus, id: "show.next-play.\(next.id)")
                                #endif
                            }
                        }
                        CastShelf(people: show.cast)
                        let seasons = Array(Set(episodes.map(\.season))).sorted()
                        Picker("Season", selection: Binding(get: { selectedSeason ?? seasons.first ?? 0 }, set: { selectedSeason = $0 })) {
                            ForEach(seasons, id: \.self) { Text($0 == 0 ? "Specials" : "Season \($0)").tag($0) }
                        }.frame(maxWidth: 420)
                        #if os(iOS)
                        if session.viewer?.downloads == true {
                            Button(downloading ? "Adding episodes…" : "Download season", systemImage: "arrow.down.circle") {
                                guard let client = session.client, !downloading else { return }
                                downloading = true
                                let chosen = episodes.filter { $0.season == (selectedSeason ?? seasons.first ?? 0) }
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
                        MediaGrid(landscape: true, items: episodes.filter { $0.season == (selectedSeason ?? seasons.first ?? 0) })
                    } else { ContentUnavailableView("No episodes", systemImage: "tv", description: Text("This show has no available episodes.")) }
                }
            }
            .padding(KinoTheme.contentPadding)
        }
        .cinemaBackground()
        #if os(tvOS)
        .focusScope(showFocus)
        #endif
        .navigationTitle("Seasons & episodes")
    }
}
