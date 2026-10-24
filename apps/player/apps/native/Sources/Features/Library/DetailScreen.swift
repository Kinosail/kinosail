import SwiftUI

struct DetailScreen: View {
    let itemID: String
    @Environment(AppSession.self) private var session
    #if os(iOS)
    @State private var scroll = SpotlightScroll()
    #endif

    var body: some View {
        #if os(tvOS)
        ScrollViewReader { scroll in
            page { scroll.scrollTo("detail.top.\(itemID)", anchor: .top) }
        }
        #else
        page {}
        #endif
    }

    private func page(onPrimaryFocus: @escaping @MainActor () -> Void) -> some View {
        ScrollView {
            ResourceView(identity: "\(session.profileKey ?? ""):\(itemID)", refreshID: session.contentRevision.uuidString, loadingLayout: .detail, focusesRetry: true, load: { policy in
                guard let client = session.client else { throw ClientError.unavailable }
                return try await client.details(id: itemID, policy: policy)
            }) { detail in
                #if os(iOS)
                DetailContent(detail: detail, scroll: scroll).id(itemID)
                #else
                DetailContent(detail: detail, onPrimaryFocus: onPrimaryFocus).id(itemID)
                #endif
            }
            .padding(KinoTheme.contentPadding)
            #if os(tvOS)
            .id("detail.top.\(itemID)")
            #endif
        }
        #if os(tvOS)
        .tvMovieHeroViewport()
        #else
        .spotlightScroll(scroll)
        #endif
        .cinemaBackground().tabBarCollapses()
        .navigationTitle("")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
    }
}

private struct DetailContent: View {
    let detail: ItemDetail
    var onPrimaryFocus: @MainActor () -> Void = {}
    #if os(iOS)
    var scroll: SpotlightScroll?
    #endif
    @Environment(AppSession.self) private var session
    @State private var listed: Bool?
    @State private var busy = false
    @State private var message: String?
    @State private var showsDownloads = false
    #if os(tvOS)
    @Namespace private var detailFocus
    @FocusState private var primaryFocused: Bool
    @Environment(\.dynamicTypeSize) private var dynamicType
    @ScaledMetric(relativeTo: .body) private var detailActionHeight = 44
    #endif
    private var item: MediaItem { detail.item }

    var body: some View {
        VStack(alignment: .leading, spacing: 28) {
            #if os(tvOS)
            if TVMovieDetail(item: item).usesBackdrop {
                tvMovieHero
                if let message { Text(message).font(.callout).foregroundStyle(KinoTheme.muted) }
            } else {
                CinemaHero(item: item, showsPlot: false, prefersEpisodeStill: true) { actions }
                information.frame(maxWidth: 900, alignment: .leading)
            }
            CastShelf(people: item.cast ?? [])
            #else
            if !item.spotlightArtwork.isEmpty {
                CinemaSpotlight(item: item, artwork: item.spotlightArtwork, scroll: scroll) { primaryAction } secondary: { secondaryActions }
            } else {
                CinemaHero(item: item, showsPlot: false, prefersEpisodeStill: true) { actions }
            }
            information
                .frame(maxWidth: 900, alignment: .leading)
            #endif
        }
        .frame(maxWidth: .infinity, alignment: .center)
        #if os(tvOS)
        .focusScope(detailFocus)
        #endif
        #if os(iOS)
        .sheet(isPresented: $showsDownloads) { NavigationStack { DownloadOptionsScreen(item: item) } }
        #endif
        .task(id: "\(session.profileKey ?? ""):\(item.id)") {
            guard [.video, .music, .audiobook].contains(item.kind), let client = session.client else { return }
            guard !Task.isCancelled else { return }
            try? await session.player.prepare(item, client: client)
        }
    }

    #if os(tvOS)
    private var tvMovieHero: some View {
        let movie = TVMovieDetail(item: item), metadata = movie.metadata
        return TVMovieHeroLayout { VStack(alignment: .leading, spacing: 14) {
            Text(movie.title)
                .font(.system(.largeTitle, design: .rounded).weight(.bold))
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityAddTraits(.isHeader)
            if !metadata.isEmpty { Text(metadata).font(.callout).foregroundStyle(KinoTheme.muted) }
            if !item.plot.isEmpty {
                Text(item.plot).font(.body).lineLimit(dynamicType.isAccessibilitySize ? nil : 4)
                    .fixedSize(horizontal: false, vertical: true)
            }
            if item.progress.seconds > 0 && !item.progress.watched { WatchPosition(item: item) }
            HStack {
                playAction
                Spacer(minLength: 0)
            }
            .focusSection()
            ViewThatFits(in: .horizontal) {
                HStack(alignment: .top, spacing: 16) { secondaryActions.fixedSize() }
                VStack(alignment: .leading, spacing: 12) { secondaryActions }
            }
            .controlSize(.large)
        } }
        .tvMovieHeroArtwork(item.backdrop)
        .foregroundStyle(KinoTheme.text)
        .tvMovieHeroBleed()
        .focusSection()
    }
    #endif

    private var information: some View {
        VStack(alignment: .leading, spacing: 20) {
            if let message { Text(message).font(.callout).foregroundStyle(.secondary) }
            if !item.plot.isEmpty { Text(item.plot).font(.body).fixedSize(horizontal: false, vertical: true) }
            if !item.genres.isEmpty { Text(item.genres).font(.callout).foregroundStyle(.secondary) }
            #if os(iOS)
            if !item.showID.isEmpty { NavigationLink("All episodes", value: ScreenDestination.show(item.showID)).frame(minHeight: 44) }
            if (item.kind == .video || item.isAudio) && !session.usesDeviceLibrary {
                NavigationLink(value: ScreenDestination.bookmarks(item.id)) { Label("Bookmarks", systemImage: "bookmark") }
                    .frame(minHeight: 44)
                NavigationLink(value: ScreenDestination.playOnTV(item.id)) { Label("Play on another device", systemImage: "tv") }
                    .frame(minHeight: 44)
            }
            if item.progress.seconds > 0 || item.progress.watched {
                Button("Remove from Continue watching") { change { client in try await client.dismissContinueWatching(itemID: item.id) } }
                    .disabled(busy)
                    .frame(minHeight: 44)
            }
            #endif
        }
    }
    @ViewBuilder private var actions: some View {
        primaryAction
        secondaryActions
    }

    @ViewBuilder private var primaryAction: some View {
        #if os(tvOS)
        if item.kind == .book {
            Label("Read on iPhone or iPad", systemImage: "iphone")
                .foregroundStyle(KinoTheme.muted)
        } else {
            playAction
        }
        #else
        playAction
        #endif
    }

    @ViewBuilder private var secondaryActions: some View {
        Button {
            change { client in listed = try await client.setListed(itemID: item.id, listed: !(listed ?? detail.listed)) }
        } label: {
            Label((listed ?? detail.listed) ? "In My List" : "My List", systemImage: (listed ?? detail.listed) ? "checkmark" : "plus")
                .contentTransition(.symbolEffect(.replace))
                #if os(tvOS)
                .frame(minHeight: detailActionHeight)
                #endif
        }
            #if os(tvOS)
            .buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
            #else
            .buttonStyle(SpotlightSecondaryButtonStyle())
            .sensoryFeedback(.success, trigger: listed) { old, new in old != nil || new != nil }
            #endif
            .disabled(busy)
        #if os(tvOS)
        if (item.kind == .video || item.isAudio) && !session.usesDeviceLibrary {
            NavigationLink(value: ScreenDestination.bookmarks(item.id)) {
                Label("Bookmarks", systemImage: "bookmark").frame(minHeight: detailActionHeight)
            }
                .buttonStyle(.bordered).buttonBorderShape(.capsule)
                .tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
        }
        if !item.showID.isEmpty {
            NavigationLink(value: ScreenDestination.show(item.showID)) {
                Text("All episodes").frame(minHeight: detailActionHeight)
            }
                .buttonStyle(.bordered).buttonBorderShape(.capsule)
                .tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
        }
        if item.progress.seconds > 0 || item.progress.watched {
            Button { change { client in try await client.dismissContinueWatching(itemID: item.id) } }
                label: { Label("Remove", systemImage: "minus.circle").frame(minHeight: detailActionHeight) }
                .accessibilityLabel("Remove from Continue watching")
                .buttonStyle(.bordered).buttonBorderShape(.capsule)
                .tint(KinoTheme.secondaryControlTint).secondaryControlForeground().disabled(busy)
        }
        #endif
        #if os(iOS)
        if session.viewer?.downloads == true && (item.kind == .video || item.isAudio) {
            DetailDownloadButton(item: item) { showsDownloads = true }
        }
        #endif
    }

    private var playAction: some View {
        NavigationLink(value: item.playingDestination) {
            Label(item.kind == .photo ? "View photo" : item.playLabel, systemImage: item.kind == .book ? "book.fill" : item.kind == .photo ? "photo" : "play.fill")
                #if os(tvOS)
                .frame(minWidth: 240, minHeight: detailActionHeight)
                .foregroundStyle(KinoTheme.tvOSPrimaryInk)
                #endif
        }
        #if os(tvOS)
        .buttonStyle(.borderedProminent).buttonBorderShape(.capsule)
        .controlSize(.large).tint(KinoTheme.tvOSPrimaryFill)
        #else
        .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
        #endif
        #if os(tvOS)
        .tvOSDefaultPlayFocus(in: detailFocus, id: "detail.play.\(item.id)", enabled: item.kind == .video || item.isAudio)
        .focused($primaryFocused)
        .onChange(of: primaryFocused) { _, focused in
            if focused && TVMovieDetail(item: item).usesBackdrop { onPrimaryFocus() }
        }
        #endif
    }
    private func change(_ operation: @escaping @MainActor (ServerClient) async throws -> Void) {
        guard let client = session.client, !busy else { return }
        busy = true
        Task {
            defer { busy = false }
            do { try await operation(client); message = nil; session.contentRevision = UUID() }
            catch { message = AppSession.message(error) }
        }
    }
}

#if os(tvOS)
struct TVMovieDetail {
    let item: MediaItem
    var usesBackdrop: Bool { item.kind == .video && !item.backdrop.isEmpty }
    var title: String { item.show.isEmpty ? item.title : ShowSeasonSelection.episodeTitle(item) }
    var metadata: String {
        let facts = item.show.isEmpty ? [item.year, item.rating, item.genres]
            : [item.show, "S\(item.season) E\(item.episode)", item.rating]
        return facts.filter { !$0.isEmpty }.joined(separator: " · ")
    }
}
#endif
