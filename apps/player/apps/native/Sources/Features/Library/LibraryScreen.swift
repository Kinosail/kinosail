import SwiftUI

struct LibraryScreen: View {
    @Environment(AppSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase
    private let searchMode: Bool
    private let searchViews: [LibraryView]?
    private let mode: PlayerMode?
    @State private var query = ""
    #if os(iOS)
    @State private var showsSearch = false
    @State private var bottomVisible = false
    #endif
    #if os(tvOS)
    @State private var quickPlay: ScreenDestination?
    @State private var focusedLetter: String?
    @State private var gridFocusPending = true
    #endif
    @State private var selection: LibraryView
    @State private var sort = LibrarySort.title
    @State private var items: [MediaItem] = []
    @State private var page: LibraryPage?
    @State private var loading = false
    @State private var failure: String?
    @State private var deferred: DeferredRetry?
    @State private var generation = UUID()
    @State private var loadedKey: String?
    @State private var loadedRevision: UUID?

    init(initialView: LibraryView = .all, searchMode: Bool = false, initialQuery: String = "", mode: PlayerMode? = nil) {
        let views = searchMode ? mode?.searchViews : nil
        _selection = State(initialValue: views?.contains(initialView) == true ? initialView : views?.first ?? initialView)
        _query = State(initialValue: initialQuery)
        self.searchMode = searchMode
        searchViews = views
        self.mode = mode
    }
    private var requestKey: String { "\(selection.rawValue):\(sort.rawValue):\(query)" }
    private var snapshotKey: String { LibrarySnapshot.key(view: selection, sort: sort, query: query) }
    private var savedSnapshot: LibrarySnapshot? {
        guard let clientID = session.client?.identity else { return nil }
        return session.resourceSnapshots.value(for: snapshotKey, clientID: clientID)
    }
    private var visibleItems: [MediaItem] { loadedKey == requestKey ? items : savedSnapshot?.items ?? [] }
    private var visiblePage: LibraryPage? { loadedKey == requestKey ? page : savedSnapshot?.page }
    private var jumpLetters: [LibraryPage.Letter] {
        guard let page = visiblePage, page.total > page.limit, page.letters.count > 1,
              sort == .title, query.isEmpty, !searchMode else { return [] }
        return page.letters
    }

    @ViewBuilder var body: some View {
        #if os(tvOS)
        if searchMode { libraryContent.searchable(text: $query, prompt: "Search your library") }
        else { libraryContent }
        #else
        libraryContent
        #endif
    }

    private var libraryContent: some View {
        ScrollViewReader { scroll in
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                #if os(tvOS)
                HStack(alignment: .firstTextBaseline) {
                    Text(searchMode ? "Search" : selection.title).font(.title2.bold()).accessibilityAddTraits(.isHeader)
                    Spacer()
                    if let page = visiblePage {
                        Text("\(page.total.formatted()) \(page.total == 1 ? "title" : "titles")")
                            .font(.callout).foregroundStyle(KinoTheme.muted)
                    }
                }
                #endif
                ViewThatFits(in: .horizontal) {
                    HStack(spacing: 16) { filters }
                    VStack(alignment: .leading, spacing: 16) { filters }
                }
                #if os(tvOS)
                .frame(maxWidth: .infinity, alignment: .leading).focusSection()
                #endif
                if visibleItems.isEmpty {
                    if visiblePage == nil && (loading || loadedKey == nil && failure == nil) {
                        #if os(iOS)
                        Text("Titles").font(.callout).redacted(reason: .placeholder).accessibilityHidden(true)
                        #endif
                        if let deferred { DeferredLoadingState(retry: deferred, layout: selection == .music || selection == .audiobooks ? .squareGrid : .grid) } else { LoadingState(layout: selection == .music || selection == .audiobooks ? .squareGrid : .grid) }
                    }
                    else if let failure { RetryState(message: failure) { Task { await load(reset: true) } } }
                    else {
                        let empty = LibraryEmptyState(view: selection, hasQuery: !query.isEmpty)
                        FeaturePlaceholder(title: empty.title, symbol: empty.symbol, message: empty.message)
                    }
                } else {
                    #if os(tvOS)
                    MediaGrid(landscape: selection == .photos, items: visibleItems, onFocus: { item in
                        if failure == nil && LibraryFocusPaging.shouldLoadNextPage(focusedID: item.id, items: visibleItems, page: visiblePage) {
                            Task { await load(reset: false) }
                        }
                    }, onQuickPlay: { quickPlay = $0 },
                    requestFirstCardFocus: !searchMode, opensShows: selection == .shows,
                    takesFirstFocus: !searchMode && gridFocusPending, onTookFocus: { gridFocusPending = false })
                    .id("library-results")
                    #else
                    if let page = visiblePage {
                        Text("\(page.total.formatted()) \(page.total == 1 ? "title" : "titles")")
                            .font(.callout).foregroundStyle(KinoTheme.muted)
                    }
                    MediaGrid(items: visibleItems, opensShows: selection == .shows).id("library-results")
                    #endif
                    if let failure { Text(failure).foregroundStyle(KinoTheme.muted) }
                    if let page = visiblePage, page.offset + page.items.count < page.total {
                        #if os(tvOS)
                        if failure != nil {
                            Button("Try again") {
                                Task {
                                    if let focusedLetter, let letter = jumpLetters.first(where: { $0.label == focusedLetter }) {
                                        await load(reset: true, start: letter.offset)
                                    } else { await load(reset: false) }
                                }
                            }
                            .tvCapsule()
                        }
                        #else
                        if loading { ProgressView("Loading more…") }
                        if failure != nil {
                            Button("Try again") { Task { await load(reset: false) } }
                                .buttonStyle(.bordered).buttonBorderShape(.capsule)
                                .tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
                        } else {
                            Color.clear.frame(height: 1).accessibilityHidden(true)
                                .onAppear {
                                    bottomVisible = true
                                    if !loading { Task { await load(reset: false) } }
                                }
                                .onDisappear { bottomVisible = false }
                                .onChange(of: loading) { _, pending in
                                    if !pending && bottomVisible && failure == nil { Task { await load(reset: false) } }
                                }
                        }
                        #endif
                    }
                }
            }
            .padding(.horizontal, KinoTheme.contentPadding).padding(.vertical, 24)
            .padding(.trailing, jumpLetters.isEmpty ? 0 : KinoTheme.letterIndexInset)
        }
        #if os(iOS)
        .overlay(alignment: .trailing) {
            if !jumpLetters.isEmpty {
                LetterScrubber(letters: jumpLetters) { letter in
                    Task { await load(reset: true, start: letter.offset); scroll.scrollTo("library-results", anchor: .top) }
                }
            }
        }
        #else
        .scrollClipDisabled()
        .overlay(alignment: .trailing) {
            if !jumpLetters.isEmpty {
                TVLetterIndex(letters: jumpLetters, onFocus: { focusedLetter = $0 }) { letter in
                    guard letter.offset != page?.offset else { return }
                    Task { await load(reset: true, start: letter.offset) }
                }.padding(.trailing, 16)
            }
        }
        .onChange(of: page?.offset) { _, _ in
            if focusedLetter != nil { scroll.scrollTo("library-results", anchor: .top) }
        }
        #endif
        }
        .cinemaBackground().tabBarCollapses()
        #if os(iOS)
        .searchable(text: $query, isPresented: $showsSearch,
                    prompt: mode == .watch ? "Search movies and shows" : mode == .listen ? "Search music and audiobooks" : "Search your library")
        #endif
        #if os(tvOS)
        .navigationTitle("")
        .navigationDestination(item: $quickPlay) { DestinationScreen(destination: $0) }
        #else
        .navigationTitle(searchMode ? "Search" : selection.title)
        #endif
        #if os(iOS)
        .onAppear { if searchMode && query.isEmpty { showsSearch = true } }
        #endif
        .task(id: "\(requestKey):\(session.contentRevision):\(scenePhase)") {
            if loadedKey != requestKey, let saved = savedSnapshot {
                items = saved.items; page = saved.page; loadedKey = requestKey; loadedRevision = saved.revision
            }
            if let clientID = session.client?.identity,
               session.resourceSnapshots.isFresh(for: snapshotKey, clientID: clientID,
                                                 as: LibrarySnapshot.self, refreshID: session.contentRevision.uuidString) { return }
            if loadedKey == requestKey, loadedRevision == session.contentRevision, let page, page.offset > 0 { return }
            do { if !query.isEmpty { try await Task.sleep(for: .milliseconds(250)) } }
            catch { return }
            await load(reset: true)
        }
        .refreshable { await load(reset: true, force: true) }
        .retrying(after: deferred) { await load(reset: true) }
        .toolbar {
            if selection == .music {
                ToolbarItem(placement: .primaryAction) {
                    NavigationLink("Albums", value: ScreenDestination.music)
                }
            }
        }
    }

    @ViewBuilder private var filters: some View {
        #if os(iOS)
        Picker("Library", selection: $selection) {
            ForEach(searchViews ?? LibraryView.allCases) { Text($0.title).tag($0) }
        }.fixedSize()
        sortPicker.fixedSize()
        #else
        if searchMode, let searchViews {
            Picker("Search in", selection: $selection) {
                ForEach(searchViews) { Text($0.title).tag($0) }
            }.pickerStyle(.menu).tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
        }
        Menu {
            sortPicker
        } label: { Label("Sort: \(sort.title)", systemImage: "arrow.up.arrow.down") }
            .tint(KinoTheme.secondaryControlTint).secondaryControlForeground()
        #endif
    }

    private var sortPicker: some View {
        Picker("Sort", selection: $sort) {
            ForEach(LibrarySort.allCases, id: \.self) { Text($0.title).tag($0) }
        }
    }

    private func load(reset: Bool, start: Int = 0, force: Bool = false) async {
        guard let client = session.client, reset || !loading else { return }
        let clientID = client.identity
        let offset = reset ? start : (page.map { $0.offset + $0.items.count } ?? 0)
        let key = requestKey
        let revision = session.contentRevision
        if reset {
            generation = UUID()
            if loadedKey != key {
                if let saved = savedSnapshot {
                    items = saved.items; page = saved.page; loadedKey = key; loadedRevision = saved.revision
                } else { items = []; page = nil; loadedKey = nil; loadedRevision = nil }
            }
        }
        let attempt = generation
        loading = true
        failure = nil
        defer { if generation == attempt { loading = false } }
        do {
            let base = reset ? [] : items
            var policy: CatalogPolicy = force ? .reload : .automatic
            if (!reset || page == nil), let saved = try? await client.library(query: query, view: selection, sort: sort, offset: offset, policy: .cached) {
                try Task.checkCancellation()
                guard attempt == generation, requestKey == key, session.client?.identity == clientID,
                      session.contentRevision == revision else { return }
                if let combined = try? Input.unique(base + saved.items) {
                    items = combined
                    page = saved; loadedKey = key
                    session.resourceSnapshots.store(LibrarySnapshot(items: combined, page: saved, revision: loadedRevision),
                                                    for: snapshotKey, clientID: clientID)
                } else { policy = .reload }
            }
            let result = try await client.library(query: query, view: selection, sort: sort, offset: offset, policy: policy)
            try Task.checkCancellation()
            guard attempt == generation, requestKey == key, session.client?.identity == clientID,
                  session.contentRevision == revision else { return }
            let combined = base + result.items
            items = try Input.unique(combined)
            page = result
            loadedKey = key; deferred = nil
            loadedRevision = revision
            session.resourceSnapshots.store(LibrarySnapshot(items: items, page: result, revision: revision),
                                            for: snapshotKey, clientID: clientID, refreshID: revision.uuidString)
        } catch is CancellationError {}
        catch {
            if generation == attempt, session.client?.identity == clientID, session.contentRevision == revision {
                if reset, let retry = DeferredRetry(error) { deferred = retry; return }
                if (error as? ClientError)?.discardsCachedContent == true {
                    items = []; page = nil
                    session.resourceSnapshots.remove(for: snapshotKey, clientID: clientID, as: LibrarySnapshot.self)
                    if error as? ClientError == .http(401) || error as? ClientError == .http(403) { session.resourceSnapshots.clear() }
                }
                failure = AppSession.message(error); deferred = nil
            }
        }
    }
}
