import SwiftUI

struct BookmarksScreen: View {
    let itemID: String
    var readingPosition: ReaderPosition? = nil
    var onReadingSelection: ((ReaderPosition) -> Void)? = nil
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    #if os(tvOS)
    @Namespace private var bookmarksFocus
    #endif
    @State private var bookmarks: [Bookmark] = []
    @State private var loaded = false
    @State private var busy = false
    @State private var message: String?
    @State private var name = ""
    @State private var revision = 0
    @State private var destination: ScreenDestination?

    var body: some View {
        List {
            #if os(tvOS)
            if !canAdd && bookmarks.isEmpty && message == nil {
                Section {
                    Button("Back", systemImage: "chevron.left") { dismiss() }
                        .tvOSDefaultPlayFocus(in: bookmarksFocus, id: "bookmarks.back")
                }
            }
            #endif
            if canAdd {
                Section("Save this position") {
                    TextField("Bookmark name", text: $name)
                    Button("Add bookmark", systemImage: "bookmark.badge.plus") { add() }.disabled(busy || name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
            if let message { Section { Text(message).foregroundStyle(.secondary); Button("Reload bookmarks") { self.message = nil; revision += 1 } } }
            if !loaded && message == nil {
                ForEach(0..<3) { index in SkeletonRow(status: index == 0 ? "Loading bookmarks…" : nil) }
            }
            else if loaded && bookmarks.isEmpty && message == nil {
                ContentUnavailableView("No bookmarks yet", systemImage: "bookmark", description: Text(emptyMessage))
            }
            ForEach(bookmarks) { bookmark in
                HStack {
                    Button { select(bookmark) } label: {
                        VStack(alignment: .leading, spacing: 6) { Text(bookmark.title).font(.headline); Text(positionLabel(bookmark.position)).font(.caption).foregroundStyle(.secondary) }
                    }.frame(maxWidth: .infinity, minHeight: 44, alignment: .leading).disabled(busy)
                    Button("Delete bookmark", systemImage: "trash", role: .destructive) { mutate { client in try await client.removeBookmark(itemID: itemID, bookmarkID: bookmark.id) } }
                        .labelStyle(.iconOnly).frame(minWidth: 44, minHeight: 44).disabled(busy)
                }
                #if os(iOS)
                .buttonStyle(.borderless)
                #endif
            }
        }
        #if os(tvOS)
        .focusScope(bookmarksFocus)
        #endif
        .tvOSConfigurationLayout(title: "Bookmarks", symbol: "bookmark")
        .configurationNavigationTitle("Bookmarks")
        .navigationDestination(item: $destination) { DestinationScreen(destination: $0) }
        .task(id: "\(session.profileKey ?? ""):\(itemID):\(revision)") {
            guard let client = session.client else { message = AppSession.message(ClientError.unavailable); return }
            do { let next = try await client.bookmarks(itemID: itemID); try Task.checkCancellation(); bookmarks = next; loaded = true; message = nil }
            catch is CancellationError {} catch { message = AppSession.message(error) }
        }
    }
    private var emptyMessage: String {
        if canAdd { return "Name this position above to save it." }
        #if os(tvOS)
        return "Save a position while playing a title."
        #else
        return "Save a position while playing or reading a title."
        #endif
    }
    private var canAdd: Bool { readingPosition != nil || session.player.currentItem?.id == itemID }
    private func positionLabel(_ position: BookmarkPosition) -> String {
        switch position { case .playback(let seconds): seconds.clock; case .reading(let page, _): "Page \(page)" }
    }
    private func add() {
        let position: BookmarkPosition
        if let readingPosition { position = .reading(page: readingPosition.page, offset: readingPosition.offset) }
        else { position = .playback(seconds: session.player.seconds) }
        mutate { client in try await client.addBookmark(itemID: itemID, title: name, position: position) }
    }
    private func select(_ bookmark: Bookmark) {
        guard let client = session.client, let store = session.progress, !busy else { return }
        busy = true
        Task {
            defer { busy = false }
            do {
                switch bookmark.position {
                case .playback(let seconds):
                    let alreadyPlaying = session.player.currentItem?.id == itemID
                    if !alreadyPlaying {
                        let item = try await client.item(id: itemID)
                        try await session.player.play(item, client: client, store: store)
                    }
                    try await session.player.seek(to: seconds)
                    if alreadyPlaying { dismiss() }
                    else { destination = session.player.currentItem?.playingDestination }
                case .reading(let page, let offset):
                    guard let readingPosition, let onReadingSelection, page <= readingPosition.total else { throw ClientError.invalidInput("Open this title in the reader to use its reading bookmarks.") }
                    onReadingSelection(ReaderPosition(page: page, total: readingPosition.total, offset: offset))
                    dismiss()
                }
            } catch { message = AppSession.message(error) }
        }
    }
    private func mutate(_ action: @escaping @MainActor (ServerClient) async throws -> [Bookmark]) {
        guard let client = session.client, !busy else { return }
        busy = true
        Task { defer { busy = false }; do { bookmarks = try await action(client); name = ""; message = nil } catch { message = AppSession.message(error) } }
    }
}
