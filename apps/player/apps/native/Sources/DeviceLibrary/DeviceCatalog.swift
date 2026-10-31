import Foundation

actor DeviceCatalog {
    private static let topLevelLifetime: TimeInterval = 600
    private static let episodeLifetime: TimeInterval = 300
    private static let activityLifetime: TimeInterval = 20
    private(set) var clients: [String: MediaSourceClient] = [:]
    private var topLevel: [DeviceItem] = []
    private var topLevelLoaded: Date?
    private var refreshing: Task<[DeviceItem], Error>?
    private var items: [String: DeviceItem] = [:]
    private var episodes: [String: (loaded: Date, items: [DeviceItem])] = [:]
    private var activity: (loaded: Date, items: [DeviceItem])?
    private var people: [String: (refs: [SourceRef], portrait: (item: String, index: Int)?)] = [:]
    private let index: DeviceIndexStore

    init(index: DeviceIndexStore) {
        self.index = index
    }

    func configure(_ accounts: [SourceAccount]) {
        clients = Dictionary(uniqueKeysWithValues: accounts.map { ($0.id, Self.client(for: $0)) })
        invalidate()
    }

    static func client(for account: SourceAccount) -> MediaSourceClient {
        switch account.kind {
        case .jellyfin, .emby: JellyfinClient(account: account)
        case .plex: PlexClient(account: account)
        }
    }

    func invalidate() {
        topLevel = []; topLevelLoaded = nil; refreshing?.cancel(); refreshing = nil
        episodes = [:]; activity = nil
    }

    func library() async throws -> [DeviceItem] {
        if let topLevelLoaded, Date().timeIntervalSince(topLevelLoaded) < Self.topLevelLifetime { return topLevel }
        if let refreshing { return try await refreshing.value }
        let clients = Array(clients.values)
        let task = Task { try await Self.collect(clients) { try await $0.topLevel() } }
        refreshing = task
        defer { refreshing = nil }
        let rows = try await task.value
        let merged = merge(rows)
        topLevel = merged
        topLevelLoaded = Date()
        return merged
    }

    func prefetch() async {
        _ = try? await recentActivity()
    }

    func recentActivity() async throws -> [DeviceItem] {
        if let activity, Date().timeIntervalSince(activity.loaded) < Self.activityLifetime { return activity.items }
        _ = try await library()
        let rows = try await Self.collect(Array(clients.values)) { try await $0.inProgress() }
        let merged = merge(rows.map(enrich)).filter { $0.state.lastPlayed != nil || $0.state.seconds > 0 }
            .sorted { ($0.state.lastPlayed ?? .distantPast) > ($1.state.lastPlayed ?? .distantPast) }
        activity = (Date(), merged)
        return merged
    }

    func search(_ query: String) async throws -> [DeviceItem] {
        _ = try await library()
        let rows = try await Self.collect(Array(clients.values)) { try await $0.search(query) }
        return merge(rows.map(enrich))
    }

    func show(_ showID: String) async throws -> (series: DeviceItem, episodes: [DeviceItem]) {
        _ = try await library()
        guard let series = try await lookup(showID), series.kind == .series else { throw DeviceFailure.notFound }
        if let cached = episodes[showID], Date().timeIntervalSince(cached.loaded) < Self.episodeLifetime { return (series, cached.items) }
        let versions = series.versions.isEmpty ? [series.ref] : series.versions
        var rows: [DeviceItem] = []
        var lastError: Error?
        for version in versions {
            guard let client = clients[version.source] else { continue }
            do { rows += try await client.episodes(series: version.item).map { enrich($0, series: series) } }
            catch { lastError = error }
        }
        if rows.isEmpty, let lastError { throw lastError }
        let merged = merge(rows).sorted { ($0.season, $0.episode, $0.sortTitle) < ($1.season, $1.episode, $1.sortTitle) }
        episodes[showID] = (Date(), merged)
        return (series, merged)
    }

    func lookup(_ id: String) async throws -> DeviceItem? {
        if let item = items[id] { return item }
        if let saved = await index.item(id), clients[saved.ref.source] != nil { items[id] = saved; return saved }
        _ = try await library()
        return items[id]
    }

    func detail(_ id: String) async throws -> DeviceItem {
        guard var item = try await lookup(id) else { throw DeviceFailure.notFound }
        guard let client = clients[item.ref.source] else { throw DeviceFailure.notFound }
        let fresh = try await client.item(item.ref.item)
        let versions = item.versions
        let identifier = item.id
        item = enrich(fresh, series: nil)
        item.id = identifier
        item.versions = versions.isEmpty ? [item.ref] : versions
        items[id] = item
        remember(item)
        await index.store([item])
        return item
    }

    func credits(_ name: String) async throws -> (items: [DeviceItem], image: String) {
        guard let person = people[name], !person.refs.isEmpty else { throw DeviceFailure.notFound }
        _ = try await library()
        var rows: [DeviceItem] = []
        for ref in person.refs {
            guard let client = clients[ref.source] else { continue }
            rows += try await client.credits(person: ref.item)
        }
        return (merge(rows.map(enrich)), person.portrait.map { "/person/\($0.item)/\($0.index)" } ?? "")
    }

    private func remember(_ item: DeviceItem) {
        for (index, person) in item.cast.enumerated() {
            guard let id = person.id else { continue }
            var entry = people[person.name] ?? (refs: [], portrait: nil)
            let ref = SourceRef(source: item.ref.source, item: id)
            if !entry.refs.contains(ref) { entry.refs.append(ref) }
            if entry.portrait == nil, person.image != nil { entry.portrait = (item.id, index) }
            people[person.name] = entry
        }
    }

    func client(for ref: SourceRef) throws -> MediaSourceClient {
        guard let client = clients[ref.source] else { throw DeviceFailure.notFound }
        return client
    }

    func updateState(_ id: String, _ change: (inout DeviceUserState) -> Void) {
        guard var item = items[id] else { return }
        change(&item.state)
        items[id] = item
        if let position = topLevel.firstIndex(where: { $0.id == id }) { topLevel[position] = item }
        activity = nil
        for (show, cached) in episodes where cached.items.contains(where: { $0.id == id }) {
            episodes[show] = (cached.loaded, cached.items.map { $0.id == id ? item : $0 })
        }
    }

    private func enrich(_ item: DeviceItem) -> DeviceItem { enrich(item, series: nil) }

    private func enrich(_ item: DeviceItem, series: DeviceItem?) -> DeviceItem {
        guard item.kind == .episode, let seriesRef = item.seriesRef else { return item }
        var item = item
        if let series, series.versions.contains(seriesRef) || series.ref == seriesRef { item.seriesProviders = series.providers; return item }
        if let match = topLevel.first(where: { $0.kind == .series && ($0.ref == seriesRef || $0.versions.contains(seriesRef)) }) {
            item.seriesProviders = match.providers
        }
        return item
    }

    private func merge(_ rows: [DeviceItem]) -> [DeviceItem] {
        var order: [String] = []
        var groups: [String: [DeviceItem]] = [:]
        for row in rows {
            let key = row.mergeKey
            if groups[key] == nil { order.append(key) }
            groups[key, default: []].append(row)
        }
        let merged = order.compactMap { key -> DeviceItem? in
            guard let group = groups[key], var best = group.max(by: { Self.richness($0) < Self.richness($1) }) else { return nil }
            best.id = DeviceItem.identifier(best.kind == .series ? best.showKey ?? key : key)
            best.versions = group.map(\.ref)
            let recent = group.max { ($0.state.lastPlayed ?? .distantPast) < ($1.state.lastPlayed ?? .distantPast) }
            best.state = recent?.state ?? best.state
            best.state.watched = recent?.state.watched ?? group.contains { $0.state.watched }
            best.state.favorite = group.contains { $0.state.favorite }
            best.width = group.map(\.width).max() ?? best.width
            return best
        }
        for item in merged { items[item.id] = item }
        Task { [index] in await index.store(merged) }
        return merged
    }

    private static func richness(_ item: DeviceItem) -> Int {
        (item.hasPrimary ? 4 : 0) + (item.hasBackdrop ? 2 : 0) + (item.plot.isEmpty ? 0 : 2) + (item.providers.isEmpty ? 0 : 1) + min(item.width / 1000, 4)
    }

    static func collect(_ clients: [MediaSourceClient], _ load: @escaping @Sendable (MediaSourceClient) async throws -> [DeviceItem]) async throws -> [DeviceItem] {
        guard !clients.isEmpty else { return [] }
        return try await withThrowingTaskGroup(of: Result<[DeviceItem], Error>.self) { group in
            for client in clients {
                group.addTask {
                    do { return .success(try await load(client)) } catch { return .failure(error) }
                }
            }
            var rows: [DeviceItem] = []
            var failure: Error?
            var succeeded = false
            for try await result in group {
                switch result {
                case .success(let batch): rows += batch; succeeded = true
                case .failure(let error):
                    failure = error
                    deviceLibraryLog.warning("operation=device-library-source outcome=failed failure_class=\(String(describing: type(of: error)), privacy: .public)")
                }
            }
            if !succeeded, let failure { throw failure }
            return rows
        }
    }
}
