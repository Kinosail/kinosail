import SwiftUI

struct HomeScreen: View {
    var showsSearch = true
    @Environment(AppSession.self) private var session
    var body: some View {
        ScrollView {
            HStack(spacing: 28) {
                Text("For you").font(.callout.weight(.semibold)).frame(minHeight: 44)
                    .overlay(alignment: .bottom) { Rectangle().fill(KinoTheme.signal).frame(height: 2) }
                    .accessibilityAddTraits(.isSelected)
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
                VStack(alignment: .leading, spacing: 32) {
                    if let featured = home.continueWatching.first ?? home.recent.first {
                        CinemaHero(item: featured, showsPlot: false) {
                            NavigationLink(value: featured.playingDestination) {
                                Label(featured.playLabel, systemImage: featured.kind == .book ? "book.fill" : "play.fill").frame(maxWidth: .infinity)
                            }.buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                            NavigationLink("Details", value: featured.destination).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
                        }
                    }
                    if !home.continueWatching.isEmpty {
                        ResumeRows(items: Array(home.continueWatching.prefix(4)))
                    }
                    if !home.recent.isEmpty { MediaShelf(title: "Recently added", items: home.recent) }
                    if home.continueWatching.isEmpty && home.recent.isEmpty {
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
        .background(KinoTheme.background)
        #if os(tvOS)
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
