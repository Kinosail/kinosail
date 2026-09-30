import SwiftUI

struct DetailScreen: View {
    let itemID: String
    @Environment(AppSession.self) private var session

    var body: some View {
        ScrollView {
            ResourceView(identity: "\(session.profileKey ?? ""):\(itemID)", refreshID: session.contentRevision.uuidString, loadingLayout: .detail, load: { policy in
                guard let client = session.client else { throw ClientError.unavailable }
                return try await client.details(id: itemID, policy: policy)
            }) { detail in DetailContent(detail: detail).id(itemID) }
            .padding(KinoTheme.contentPadding)
        }
        .cinemaBackground()
        .navigationTitle("")
        #if os(iOS)
        .navigationBarTitleDisplayMode(.inline)
        #endif
    }
}

private struct DetailContent: View {
    let detail: ItemDetail
    @Environment(AppSession.self) private var session
    @State private var listed: Bool?
    @State private var busy = false
    @State private var message: String?
    @State private var showsDownloads = false
    #if os(tvOS)
    @Namespace private var detailFocus
    @Environment(\.colorSchemeContrast) private var contrast
    @Environment(\.accessibilityReduceTransparency) private var reduceTransparency
    @Environment(\.dynamicTypeSize) private var dynamicType
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
            CinemaHero(item: item, showsPlot: false, prefersEpisodeStill: true) { actions }
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
        let metadata = TVMovieDetail(item: item).metadata
        return VStack(alignment: .leading, spacing: 14) {
            Text(item.title)
                .font(.system(.largeTitle, design: .rounded).weight(.bold))
                .fixedSize(horizontal: false, vertical: true)
                .accessibilityAddTraits(.isHeader)
            if !metadata.isEmpty { Text(metadata).font(.callout).foregroundStyle(KinoTheme.muted) }
            if !item.plot.isEmpty {
                Text(item.plot).font(.body).lineLimit(dynamicType.isAccessibilitySize ? nil : 4)
                    .fixedSize(horizontal: false, vertical: true)
            }
            if item.progress.seconds > 0 && !item.progress.watched { WatchPosition(item: item) }
            playAction
            ViewThatFits(in: .horizontal) {
                HStack(spacing: 16) { secondaryActions.fixedSize() }
                VStack(alignment: .leading, spacing: 12) { secondaryActions }
            }
            .controlSize(.large)
        }
        .frame(maxWidth: 740, alignment: .leading)
        .padding(40)
        .frame(maxWidth: .infinity, minHeight: 520, alignment: .bottomLeading)
        .background {
            ZStack {
                KinoTheme.surface
                if contrast != .increased && !reduceTransparency {
                    GeometryReader { geometry in
                        Artwork(path: item.backdrop, ratio: 16 / 9, dimension: 1600,
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
    #endif

    private var information: some View {
        VStack(alignment: .leading, spacing: 20) {
            if let message { Text(message).font(.callout).foregroundStyle(.secondary) }
            if !item.plot.isEmpty { Text(item.plot).font(.body).fixedSize(horizontal: false, vertical: true) }
            if !item.genres.isEmpty { Text(item.genres).font(.callout).foregroundStyle(.secondary) }
            #if os(iOS)
            if !item.showID.isEmpty { NavigationLink("All episodes", value: ScreenDestination.show(item.showID)).frame(minHeight: 44) }
            if item.kind == .video || item.isAudio {
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
        } label: { Label((listed ?? detail.listed) ? "In My List" : "My List", systemImage: (listed ?? detail.listed) ? "checkmark" : "plus") }
            .buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground().disabled(busy)
        #if os(tvOS)
        if item.kind == .video || item.isAudio {
            NavigationLink(value: ScreenDestination.bookmarks(item.id)) { Label("Bookmarks", systemImage: "bookmark") }
                .buttonStyle(.bordered).buttonBorderShape(.capsule)
                .tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
        }
        if !item.showID.isEmpty {
            NavigationLink("All episodes", value: ScreenDestination.show(item.showID))
                .buttonStyle(.bordered).buttonBorderShape(.capsule)
                .tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
        }
        if item.progress.seconds > 0 || item.progress.watched {
            Button { change { client in try await client.dismissContinueWatching(itemID: item.id) } }
                label: { Label("Remove", systemImage: "minus.circle") }
                .accessibilityLabel("Remove from Continue watching")
                .buttonStyle(.bordered).buttonBorderShape(.capsule)
                .tint(KinoTheme.secondaryControlTint).secondaryControlForeground().disabled(busy)
        }
        #endif
        #if os(iOS)
        if session.viewer?.downloads == true && (item.kind == .video || item.isAudio) {
            Button { showsDownloads = true } label: { Label("Download", systemImage: "arrow.down") }.buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
        }
        #endif
    }

    private var playAction: some View {
        NavigationLink(value: item.playingDestination) {
            Label(item.kind == .photo ? "View photo" : item.playLabel, systemImage: item.kind == .book ? "book.fill" : item.kind == .photo ? "photo" : "play.fill")
                #if os(tvOS)
                .frame(minWidth: 240).padding(.vertical, 12)
                .foregroundStyle(KinoTheme.tvOSPrimaryInk)
                .background(KinoTheme.tvOSPrimaryFill, in: Capsule())
                #endif
        }
        #if os(tvOS)
        .buttonStyle(.card)
        #else
        .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
        #endif
        #if os(tvOS)
        .tvOSDefaultPlayFocus(in: detailFocus, id: "detail.play.\(item.id)", enabled: item.kind == .video || item.isAudio)
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
    var usesBackdrop: Bool {
        item.kind == .video && item.show.isEmpty && item.showID.isEmpty && !item.backdrop.isEmpty
    }
    var metadata: String { [item.year, item.rating, item.genres].filter { !$0.isEmpty }.joined(separator: " · ") }
}
#endif
