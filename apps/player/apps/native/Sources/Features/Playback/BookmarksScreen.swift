import SwiftUI

struct BookmarksScreen: View {
    let itemID: String
    var readingPosition: ReaderPosition? = nil
    var onReadingSelection: ((ReaderPosition) -> Void)? = nil
    @Environment(AppSession.self) private var session
    @Environment(\.dismiss) private var dismiss
    @State private var bookmarks: [Bookmark] = []
    @State private var loaded = false
    @State private var busy = false
    @State private var message: String?
    @State private var name = ""
    @State private var revision = 0
    @State private var destination: ScreenDestination?

    var body: some View {
        List {
            if canAdd {
                Section("Save this position") {
                    TextField("Bookmark name", text: $name)
                    Button("Add bookmark", systemImage: "bookmark.badge.plus") { add() }.disabled(busy || name.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty)
                }
            }
            if let message { Section { Text(message).foregroundStyle(.secondary); Button("Reload bookmarks") { self.message = nil; revision += 1 } } }
            if !loaded && message == nil { ProgressView("Loading bookmarks…") }
            else if loaded && bookmarks.isEmpty && message == nil {
                ContentUnavailableView("No bookmarks yet", systemImage: "bookmark", description: Text("Save a position while playing or reading a title."))
            }
            ForEach(bookmarks) { bookmark in
                HStack {
                    Button { select(bookmark) } label: {
                        VStack(alignment: .leading, spacing: 6) { Text(bookmark.title).font(.headline); Text(positionLabel(bookmark.position)).font(.caption).foregroundStyle(.secondary) }
                    }.frame(maxWidth: .infinity, alignment: .leading).disabled(busy)
                    Button("Delete bookmark", systemImage: "trash", role: .destructive) { mutate { client in try await client.removeBookmark(itemID: itemID, bookmarkID: bookmark.id) } }
                        .labelStyle(.iconOnly).disabled(busy)
                }
            }
        }
        .tvOSConfigurationLayout(title: "Bookmarks", symbol: "bookmark")
        .navigationTitle("Bookmarks")
        .navigationDestination(item: $destination) { DestinationScreen(destination: $0) }
        .task(id: "\(session.profileKey ?? ""):\(itemID):\(revision)") {
            guard let client = session.client else { return }
            do { let next = try await client.bookmarks(itemID: itemID); try Task.checkCancellation(); bookmarks = next; loaded = true; message = nil }
            catch is CancellationError {} catch { message = AppSession.message(error) }
        }
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

struct ProgressSyncScreen: View {
    @Environment(AppSession.self) private var session
    @State private var entries: [PendingProgress] = []
    @State private var message: String?
    @State private var busy = false
    @State private var loaded = false

    var body: some View {
        List {
            if let message {
                Text(message).foregroundStyle(.secondary)
                Button("Try again") { Task { await synchronize() } }.disabled(busy)
            }
            if !loaded && message == nil { ProgressView("Loading saved progress…") }
            else if busy { ProgressView("Syncing saved progress…") }
            else if loaded && entries.isEmpty && message == nil {
                ContentUnavailableView("Progress is up to date", systemImage: "checkmark.circle", description: Text("All saved playback positions have synced."))
            }
            ForEach(entries) { entry in
                Section {
                    NavigationLink("Open title", value: ScreenDestination.detail(entry.itemID))
                    LabeledContent("This device", value: entry.progress.seconds.clock)
                    if let remote = entry.conflict {
                        LabeledContent("Server", value: remote.seconds.clock)
                        Text("Another device changed this title’s progress. Choose which position to keep.").foregroundStyle(.secondary)
                        Button("Keep this device’s position") { resolve(entry, useDevice: true) }.disabled(busy)
                        Button("Keep the Server’s position") { resolve(entry, useDevice: false) }.disabled(busy)
                    } else { Text("Waiting to sync").foregroundStyle(.secondary) }
                }
            }
            if !entries.isEmpty { Button("Sync now") { Task { await synchronize() } }.disabled(busy) }
        }
        .tvOSConfigurationLayout(title: "Progress sync", symbol: "arrow.triangle.2.circlepath")
        .navigationTitle("Progress sync")
        .task { await synchronize() }
    }
    private func synchronize() async {
        guard let store = session.progress, let client = session.client, !busy else { return }
        busy = true
        message = nil
        defer { busy = false }
        do { entries = try await store.pending(); _ = try await store.synchronize(client: client); entries = try await store.pending(); loaded = true }
        catch { message = AppSession.message(error) }
    }
    private func resolve(_ entry: PendingProgress, useDevice: Bool) {
        guard let store = session.progress, !busy else { return }
        busy = true
        Task {
            do { try await store.resolve(itemID: entry.itemID, useDevice: useDevice); entries = try await store.pending(); message = nil }
            catch { message = AppSession.message(error) }
            busy = false
            if useDevice { await synchronize() }
            session.contentRevision = UUID()
        }
    }
}
