import SwiftUI

struct LibraryScreen: View {
    @Environment(AppSession.self) private var session
    @Environment(\.scenePhase) private var scenePhase
    private let searchMode: Bool
    @State private var query = ""
    @State private var showsSearch = false
    #if os(iOS)
    @State private var showsLetterJump = false
    #endif
    @State private var selection: LibraryView
    @State private var sort = LibrarySort.title
    @State private var items: [MediaItem] = []
    @State private var page: LibraryPage?
    @State private var loading = false
    @State private var failure: String?
    @State private var generation = UUID()
    @State private var loadedKey: String?
    #if os(tvOS)
    @State private var draftQuery = ""
    @State private var focusedItem: MediaItem?
    @State private var backdropItem: MediaItem?
    #endif
    @State private var loadedRevision: UUID?

    init(initialView: LibraryView = .all, searchMode: Bool = false, initialQuery: String = "") {
        _selection = State(initialValue: initialView)
        _query = State(initialValue: initialQuery)
        #if os(tvOS)
        _draftQuery = State(initialValue: initialQuery)
        #endif
        self.searchMode = searchMode
    }
    private var requestKey: String { "\(selection.rawValue):\(sort.rawValue):\(query)" }

    var body: some View {
        ScrollView {
            VStack(alignment: .leading, spacing: 24) {
                #if os(tvOS)
                HStack(alignment: .firstTextBaseline) {
                    Text(searchMode ? "Search" : selection.title).font(.title2.bold()).accessibilityAddTraits(.isHeader)
                    Spacer()
                    if let page { Text("\(page.total.formatted()) titles").font(.callout).foregroundStyle(KinoTheme.muted) }
                }
                #endif
                ViewThatFits(in: .horizontal) {
                    HStack(spacing: 16) { filters }
                    VStack(alignment: .leading, spacing: 16) { filters }
                }
                if items.isEmpty {
                    if page == nil && (loading || loadedKey == nil && failure == nil) { LoadingState(layout: .grid) }
                    else if let failure { RetryState(message: failure) { Task { await load(reset: true) } } }
                    else { FeaturePlaceholder(title: query.isEmpty ? "Nothing here yet" : "No matches", symbol: "magnifyingglass",
                                              message: query.isEmpty ? "Try another part of your library." : "Try a different title, artist or show.") }
                } else {
                    #if os(tvOS)
                    if let featured = backdropItem ?? items.first {
                        HStack(spacing: 24) {
                            if !featured.backdrop.isEmpty {
                                Artwork(path: featured.backdrop, ratio: 16 / 9, isBackdrop: true)
                                    .frame(width: 320).clipShape(.rect(cornerRadius: 12))
                            }
                            VStack(alignment: .leading, spacing: 8) {
                                Text(featured.title).font(.title2.weight(.semibold)).fixedSize(horizontal: false, vertical: true)
                                Text(featured.subtitle).font(.callout).foregroundStyle(KinoTheme.muted)
                            }
                        }.frame(maxWidth: .infinity, minHeight: 180, alignment: .leading).accessibilityHidden(true)
                    }
                    MediaGrid(items: items, onFocus: { focusedItem = $0 })
                    #else
                    if let page { Text("\(page.total.formatted()) titles").font(.callout).foregroundStyle(KinoTheme.muted) }
                    MediaGrid(items: items)
                    #endif
                    if let failure { Text(failure).foregroundStyle(KinoTheme.muted) }
                    if let page, page.offset + page.items.count < page.total {
                        Button(loading ? "Loading more…" : "Load more") { Task { await load(reset: false) } }
                            .buttonStyle(.bordered).buttonBorderShape(.capsule).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text).disabled(loading)
                    }
                }
            }
            .padding(.horizontal, KinoTheme.contentPadding)
            .padding(.vertical, 24)
        }
        #if os(tvOS)
        .scrollClipDisabled()
        .background(KinoTheme.background)
        .task(id: focusedItem?.id) {
            guard let focusedItem else { return }
            do { try await Task.sleep(for: .milliseconds(300)) }
            catch { return }
            backdropItem = focusedItem
        }
        #else
        .background(KinoTheme.background)
        #endif
        #if os(iOS)
        .searchable(text: $query, isPresented: $showsSearch, prompt: "Search your library")
        .sheet(isPresented: $showsLetterJump) {
            LetterJumpSheet(letters: page?.letters ?? []) { letter in
                Task { await load(reset: true, start: letter.offset) }
            }
        }
        #else
        .sheet(isPresented: $showsSearch) {
            NavigationStack {
                Form {
                    TextField("Search your library", text: $draftQuery).onSubmit { query = draftQuery; showsSearch = false }
                    Button("Search") { query = draftQuery; showsSearch = false }
                        .buttonStyle(.borderedProminent).tint(KinoTheme.signal).foregroundStyle(KinoTheme.signalInk)
                    Button("Cancel") { showsSearch = false }.tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
                }.navigationTitle("Search library")
            }
        }
        #endif
        #if os(tvOS)
        .navigationTitle("")
        #else
        .navigationTitle(searchMode ? "Search" : selection.title)
        #endif
        .onAppear { if searchMode && query.isEmpty { showsSearch = true } }
        .task(id: "\(requestKey):\(session.contentRevision):\(scenePhase)") {
            guard scenePhase == .active else { return }
            // Preserve an expanded or letter-jump result when returning to it.
            if loadedKey == requestKey, loadedRevision == session.contentRevision, let page, page.offset > 0 { return }
            do { if !query.isEmpty { try await Task.sleep(for: .milliseconds(250)) } }
            catch { return }
            await load(reset: true)
        }
        .refreshable { await load(reset: true, force: true) }
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
            ForEach(LibraryView.allCases) { Text($0.title).tag($0) }
        }
        #else
        Button { draftQuery = query; showsSearch = true } label: { Label("Search", systemImage: "magnifyingglass") }
            .buttonStyle(.bordered).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
        NavigationLink { LibraryHubScreen() } label: { Label("Browse library", systemImage: "square.grid.2x2") }
            .buttonStyle(.bordered).tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
        #endif
        #if os(tvOS)
        Menu {
            sortPicker
        } label: { Label("Sort: \(sort.title)", systemImage: "arrow.up.arrow.down") }
            .tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
        #else
        sortPicker
        #endif
        if let page, !page.letters.isEmpty, sort == .title {
            #if os(iOS)
            Button { showsLetterJump = true } label: {
                Label("A–Z", systemImage: "textformat.abc")
            }
            .buttonStyle(.bordered)
            .buttonBorderShape(.capsule)
            .tint(KinoTheme.secondaryControlTint)
            .foregroundStyle(KinoTheme.text)
            #else
            Menu("Jump to letter", systemImage: "textformat.abc") {
                ForEach(page.letters) { letter in
                    Button("\(letter.label) · \(letter.count)") { Task { await load(reset: true, start: letter.offset) } }
                }
            }.tint(KinoTheme.secondaryControlTint).foregroundStyle(KinoTheme.text)
            #endif
        }
    }

    private var sortPicker: some View {
        Picker("Sort", selection: $sort) {
            ForEach(LibrarySort.allCases, id: \.self) { Text($0.title).tag($0) }
        }
    }

    private func load(reset: Bool, start: Int = 0, force: Bool = false) async {
        guard let client = session.client, reset || !loading else { return }
        let offset = reset ? start : (page.map { $0.offset + $0.items.count } ?? 0)
        let key = requestKey
        let revision = session.contentRevision
        if reset {
            generation = UUID()
            if loadedKey != key {
                items = []; page = nil
            #if os(tvOS)
                focusedItem = nil; backdropItem = nil
            #endif
            }
        }
        let attempt = generation
        loading = true
        failure = nil
        defer { if generation == attempt { loading = false } }
        do {
            let base = reset ? [] : items
            var policy: CatalogPolicy = force ? .reload : .automatic
            if let saved = try? await client.library(query: query, view: selection, sort: sort, offset: offset, policy: .cached) {
                try Task.checkCancellation()
                guard attempt == generation, requestKey == key else { return }
                if let combined = try? Input.unique(base + saved.items) {
                    items = combined
                    page = saved; loadedKey = key
                } else { policy = .reload }
            }
            let result = try await client.library(query: query, view: selection, sort: sort, offset: offset, policy: policy)
            try Task.checkCancellation()
            guard attempt == generation, requestKey == key else { return }
            let combined = base + result.items
            items = try Input.unique(combined)
            page = result
            loadedKey = key
            loadedRevision = revision
        } catch is CancellationError {}
        catch {
            if generation == attempt {
                if (error as? ClientError)?.discardsCachedContent == true { items = []; page = nil }
                failure = AppSession.message(error)
            }
        }
    }
}

#if os(iOS)
private struct LetterJumpSheet: View {
    let letters: [LibraryPage.Letter]
    let onSelect: (LibraryPage.Letter) -> Void
    @Environment(\.dismiss) private var dismiss

    var body: some View {
        NavigationStack {
            ScrollView {
                VStack(alignment: .leading, spacing: 16) {
                    Text("Choose a starting letter. Titles stay sorted A–Z.")
                        .font(.subheadline)
                        .foregroundStyle(KinoTheme.muted)

                    LazyVGrid(columns: [GridItem(.adaptive(minimum: 76), spacing: 12)], spacing: 12) {
                        ForEach(letters) { letter in
                            Button {
                                onSelect(letter)
                                dismiss()
                            } label: {
                                VStack(spacing: 4) {
                                    Text(letter.label)
                                        .font(.title3.weight(.semibold))
                                    Text("\(letter.count) \(letter.count == 1 ? "title" : "titles")")
                                        .font(.caption)
                                        .foregroundStyle(KinoTheme.muted)
                                }
                                .frame(maxWidth: .infinity, minHeight: 72)
                            }
                            .buttonStyle(.bordered)
                            .buttonBorderShape(.roundedRectangle(radius: 14))
                            .tint(KinoTheme.secondaryControlTint)
                            .foregroundStyle(KinoTheme.text)
                        }
                    }
                }
                .padding(KinoTheme.contentPadding)
            }
            .background(KinoTheme.background)
            .navigationTitle("Browse by title")
            .toolbar {
                ToolbarItem(placement: .cancellationAction) {
                    Button("Done") { dismiss() }
                }
            }
        }
        .presentationDetents([.medium, .large])
        .presentationDragIndicator(.visible)
    }
}
#endif
