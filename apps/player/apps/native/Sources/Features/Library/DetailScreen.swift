import SwiftUI

struct DetailScreen: View {
    let itemID: String
    @Environment(AppSession.self) private var session

    var body: some View {
        ScrollView {
            ResourceView(identity: "\(session.profileKey ?? ""):\(itemID)", refreshID: session.contentRevision.uuidString, loadingLayout: .detail, load: { policy in
                guard let client = session.client else { throw ClientError.unavailable }
                return try await client.details(id: itemID, policy: policy)
            }) { detail in DetailContent(detail: detail).id(itemID).cinemaBackdrop(path: detail.item.backdrop) }
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
    #endif
    private var item: MediaItem { detail.item }

    var body: some View {
        VStack(alignment: .leading, spacing: 28) {
            CinemaHero(item: item, showsPlot: false) { actions }
            information
                .frame(maxWidth: 900, alignment: .leading)
                #if os(tvOS)
                .buttonStyle(.bordered).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
                #endif
            CastShelf(people: item.cast ?? [])
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
            try? await Task.sleep(for: .milliseconds(250))
            guard !Task.isCancelled else { return }
            try? await session.player.prepare(item, client: client)
        }
    }

    private var information: some View {
        VStack(alignment: .leading, spacing: 20) {
            if let message { Text(message).font(.callout).foregroundStyle(.secondary) }
            if !item.plot.isEmpty { Text(item.plot).font(.body).fixedSize(horizontal: false, vertical: true) }
            if !item.genres.isEmpty { Text(item.genres).font(.callout).foregroundStyle(.secondary) }
            if !item.showID.isEmpty { NavigationLink("All episodes", value: ScreenDestination.show(item.showID)) }
            if item.kind == .video || item.isAudio {
                NavigationLink(value: ScreenDestination.bookmarks(item.id)) { Label("Bookmarks", systemImage: "bookmark") }
                NavigationLink(value: ScreenDestination.playOnTV(item.id)) { Label("Play on TV", systemImage: "tv") }
            }
            if item.progress.seconds > 0 || item.progress.watched {
                Button("Remove from Continue watching") { change { client in try await client.dismissContinueWatching(itemID: item.id) } }
                    .disabled(busy)
            }
        }
    }
    @ViewBuilder private var actions: some View {
        NavigationLink(value: item.playingDestination) {
            Label(item.kind == .photo ? "View photo" : item.playLabel, systemImage: item.kind == .book ? "book.fill" : item.kind == .photo ? "photo" : "play.fill")
        }
        .buttonStyle(.borderedProminent).buttonBorderShape(.capsule).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
        #if os(tvOS)
        .prefersDefaultFocus(item.kind == .video, in: detailFocus)
        #endif
        Button {
            change { client in listed = try await client.setListed(itemID: item.id, listed: !(listed ?? detail.listed)) }
        } label: { Label((listed ?? detail.listed) ? "In My List" : "My List", systemImage: (listed ?? detail.listed) ? "checkmark" : "plus") }
            .buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text).disabled(busy)
        #if os(iOS)
        if session.viewer?.downloads == true && (item.kind == .video || item.isAudio) {
            Button { showsDownloads = true } label: { Label("Download", systemImage: "arrow.down") }.buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
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
