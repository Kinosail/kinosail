import SwiftUI

struct HomeScreen: View {
    var showsSearch = true
    @Environment(AppSession.self) private var session
    #if os(tvOS)
    @Namespace private var homeFocus
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
            ResourceView(identity: session.profileKey ?? "", refreshID: session.contentRevision.uuidString, loadingLayout: .home, allowsPullToRefresh: false, load: { policy in
                guard let client = session.client else { throw ClientError.http(401) }
                return try await client.home(policy: policy)
            }) { home in
                let selection = HomeSelection(continueWatching: home.continueWatching, recent: home.recent)
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
                    if !selection.continuation.isEmpty { ResumeRows(items: selection.continuation) }
                    if !selection.recent.isEmpty { MediaShelf(title: "Recently added", items: selection.recent) }
                    if selection.featured == nil {
                        FeaturePlaceholder(title: "Your library is ready", symbol: "play.rectangle",
                                           message: "Media added to your Server will appear here.")
                    }
                    VStack(alignment: .leading, spacing: 12) {
                        Text("Browse library").font(.title2.bold()).accessibilityAddTraits(.isHeader)
                        LibraryQuickLinks()
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
        #else
        .navigationTitle("Kinosail")
        #endif
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
        .toolbar {
            if showsSearch {
                ToolbarItem { NavigationLink(value: ScreenDestination.search) { Image(systemName: "magnifyingglass").accessibilityLabel("Search library") } }
            }
        }
    }
}

struct HomeSelection {
    let featured: MediaItem?
    let continuation: [MediaItem]
    let recent: [MediaItem]

    init(continueWatching: [MediaItem], recent: [MediaItem]) {
        featured = continueWatching.first ?? recent.first
        continuation = Array(continueWatching.dropFirst().prefix(4))
        let visibleIDs = Set(([featured].compactMap { $0 } + continuation).map(\.id))
        self.recent = recent.filter { !visibleIDs.contains($0.id) }
    }
}
