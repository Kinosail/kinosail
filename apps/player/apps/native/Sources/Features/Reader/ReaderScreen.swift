import SwiftUI
#if os(iOS)
import UIKit
#endif

private struct ReaderLoadingState: View {
    var body: some View {
        VStack(alignment: .leading, spacing: 18) {
            ForEach(0..<7) { index in
                RoundedRectangle(cornerRadius: 4).fill(KinoTheme.surface)
                    .frame(maxWidth: index == 6 ? 220 : .infinity).frame(height: 18)
            }
            Spacer()
            HStack {
                Circle().fill(KinoTheme.raised).frame(width: 44, height: 44)
                Spacer()
                Capsule().fill(KinoTheme.raised).frame(width: 120, height: 44)
                Spacer()
                Circle().fill(KinoTheme.raised).frame(width: 44, height: 44)
            }
        }
        .padding(KinoTheme.contentPadding)
        .frame(maxWidth: .infinity, maxHeight: .infinity, alignment: .topLeading)
        .skeletonLoading("Opening your book…")
        .background(KinoTheme.background)
    }
}

struct ReaderScreen: View {
    let itemID: String
    @Environment(AppSession.self) private var session
    #if os(iOS)
    @Environment(\.colorScheme) private var colorScheme
    @Environment(\.dynamicTypeSize) private var dynamicType
    @State private var book: ReaderBook?
    @State private var position = ReaderPosition(page: 1, total: 1, offset: 0)
    @State private var preferences = MediaPreferences()
    @State private var failure: String?
    @State private var notice: String?
    @State private var loadRevision = 0
    @State private var jump = UUID()
    @State private var showsContents = false
    @State private var showsBookmarks = false
    @State private var showsPreferences = false
    @State private var writer: ReaderProgressWriter?
    @State private var conflict: ReaderPosition?
    @State private var saveTask: Task<Void, Never>?
    @State private var generation = UUID()
    #endif

    var body: some View {
        #if os(iOS)
        Group {
            if let failure { RetryState(message: failure) { loadRevision += 1 } }
            else if let book, let client = session.client, book.pages.indices.contains(position.page - 1) {
                ReaderDocument(book: book, page: book.pages[position.page - 1], position: position, fontSize: displayFont,
                               theme: preferences.readerTheme == .auto ? (colorScheme == .dark ? .dark : .light) : preferences.readerTheme,
                               revision: jump, client: client, onOffset: { offset in
                    position = ReaderPosition(page: position.page, total: position.total, offset: offset)
                    save()
                }, onFailure: { notice = $0 }, onNavigate: { url in
                    if let page = book.pages.first(where: { $0.resource == url }) { turn(to: page.number) }
                })
                .safeAreaInset(edge: .bottom, spacing: 0) {
                    VStack(spacing: 10) {
                        if let notice { Text(notice).font(.caption).foregroundStyle(.secondary); Button("Reopen chapter") { self.notice = nil; jump = UUID() } }
                        if let conflict {
                            Text("The Server has a different reading position.").font(.callout)
                            VStack(alignment: .leading, spacing: 8) {
                                Button("Keep this device’s position") { chooseDevice(conflict) }
                                Button("Use the Server’s position") { chooseServer(conflict) }
                            }.font(.callout)
                        }
                        HStack {
                            Button("Previous", systemImage: "chevron.left") { turn(to: position.page - 1) }.labelStyle(.iconOnly).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.secondaryControlInk).controlSize(.large).disabled(position.page <= 1)
                            Spacer()
                            Button(book.kind == .pdf ? "Document" : "\(book.kind == .epub ? "Chapter" : "Page") \(position.page) of \(position.total)") { showsContents = true }.frame(minHeight: 44)
                            Spacer()
                            Button("Next", systemImage: "chevron.right") { turn(to: position.page + 1) }.labelStyle(.iconOnly).buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.secondaryControlInk).controlSize(.large).disabled(position.page >= position.total)
                        }
                    }.padding(16).background(.regularMaterial)
                }
            } else { ReaderLoadingState() }
        }
        .navigationTitle(book?.title ?? "Reader")
        .navigationBarTitleDisplayMode(.inline)
        .toolbar(.hidden, for: .tabBar)
        .toolbar {
            ToolbarItem(placement: .primaryAction) {
                Menu("Reading options", systemImage: "ellipsis.circle") {
                    Button("Contents", systemImage: "list.bullet") { showsContents = true }
                    Button("Bookmarks", systemImage: "bookmark") { showsBookmarks = true }
                    Button("Appearance", systemImage: "textformat.size") { showsPreferences = true }
                }
            }
        }
        .sheet(isPresented: $showsContents) {
            NavigationStack {
                List(book?.pages ?? []) { page in Button(page.title.isEmpty ? "Page \(page.number)" : page.title) { turn(to: page.number); showsContents = false } }
                    .navigationTitle("Contents").toolbar { ToolbarItem(placement: .cancellationAction) { Button("Done") { showsContents = false } } }
            }
        }
        .sheet(isPresented: $showsBookmarks) { NavigationStack { BookmarksScreen(itemID: itemID, readingPosition: position) { newPosition in position = newPosition; jump = UUID(); save() } } }
        .sheet(isPresented: $showsPreferences, onDismiss: { Task { await reloadPreferences() } }) { NavigationStack { ReaderPreferencesScreen() } }
        .task(id: "\(session.profileKey ?? ""):\(itemID):\(loadRevision)") { await load() }
        .onAppear { session.reading = true }
        .onDisappear { session.reading = false; save() }
        #else
        ContentUnavailableView("Read on iPhone or iPad", systemImage: "book", description: Text("Open this title in Kinosail on your iPhone or iPad."))
        #endif
    }
    #if os(iOS)
    private var displayFont: Int { _ = dynamicType; return min(96, max(16, Int(UIFontMetrics(forTextStyle: .body).scaledValue(for: CGFloat(preferences.readerFontSize))))) }
    private func load() async {
        let attempt = UUID(); generation = attempt
        failure = nil; notice = nil; book = nil; writer = nil; conflict = nil
        guard let client = session.client, let scope = session.profileKey else { return }
        do {
            async let details = client.reader(itemID: itemID)
            async let remotePosition = client.readerPosition(itemID: itemID)
            async let savedPreferences = client.mediaPreferences()
            let (book, remote, preferences) = try await (details, remotePosition, savedPreferences)
            guard remote.total == book.pages.count else { throw ClientError.invalidResponse }
            let store = try ReaderPositionStore(scope: scope)
            let pending = try await store.pending(itemID: itemID)
            try Task.checkCancellation()
            guard generation == attempt else { return }
            self.book = book; self.preferences = preferences; self.position = remote
            writer = try ReaderProgressWriter(itemID: itemID, client: client, store: store, expected: remote)
            if let pending, pending.position.total == remote.total {
                position = pending.position
                if remote != pending.expected && remote != pending.position { conflict = remote }
                else { save() }
            } else if pending != nil { notice = "The book’s page order changed. The Server’s current position is shown." }
            jump = UUID()
        } catch is CancellationError {} catch { if generation == attempt { failure = AppSession.message(error) } }
    }
    private func reloadPreferences() async {
        guard let client = session.client else { return }
        do { preferences = try await client.mediaPreferences(); jump = UUID() } catch { notice = AppSession.message(error) }
    }
    private func turn(to page: Int) {
        guard (1...position.total).contains(page) else { return }
        position = ReaderPosition(page: page, total: position.total, offset: 0); jump = UUID(); notice = nil
        save()
    }
    private func save() {
        guard let writer, book != nil, conflict == nil else { return }
        let position = position, attempt = generation
        saveTask = Task {
            do {
                let result = try await writer.update(position)
                guard generation == attempt else { return }
                switch result {
                case .saved: notice = nil
                case .queued: break
                case .offline: notice = "Your reading position is saved on this device. It will sync when the Server is available."
                case .conflict(let remote): conflict = remote
                }
            } catch is CancellationError {} catch { if generation == attempt { notice = "Couldn’t save the reading position. \(AppSession.message(error))" } }
        }
    }
    private func chooseDevice(_ remote: ReaderPosition) {
        guard let writer else { return }
        Task {
            do { _ = try await writer.chooseDevice(over: remote); conflict = nil; save() }
            catch { notice = AppSession.message(error) }
        }
    }
    private func chooseServer(_ remote: ReaderPosition) {
        guard let writer else { return }
        Task { do { try await writer.chooseServer(remote); position = remote; conflict = nil; jump = UUID() } catch { notice = AppSession.message(error) } }
    }
    #endif
}

#if os(iOS)
private struct ReaderDocument: UIViewRepresentable {
    let book: ReaderBook
    let page: ReaderBook.Page
    let position: ReaderPosition
    let fontSize: Int
    let theme: ReaderTheme
    let revision: UUID
    let client: ServerClient
    let onOffset: (Double) -> Void
    let onFailure: (String) -> Void
    let onNavigate: (URL) -> Void
    func makeUIView(context: Context) -> ProtectedReaderView { ProtectedReaderView() }
    func updateUIView(_ view: ProtectedReaderView, context: Context) {
        view.onOffset = onOffset; view.onFailure = onFailure; view.onNavigate = onNavigate
        view.load(book: book, page: page, offset: position.offset, fontSize: fontSize, theme: theme, revision: revision, client: client)
    }
    static func dismantleUIView(_ view: ProtectedReaderView, coordinator: ()) { view.close() }
}
#endif
