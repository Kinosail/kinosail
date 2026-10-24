import Foundation

struct PlexClient: MediaSourceClient {
    let account: SourceAccount
    private static let pageSize = 300
    private static let library = "com.plexapp.plugins.library"

    func request(_ path: String, _ query: [String: String] = [:], method: String = "GET", base: URL? = nil) throws -> URLRequest {
        var request = URLRequest(url: try SourceHTTP.url(base ?? PlexRoutes.shared.current(account), path, query))
        request.httpMethod = method
        PlexAccounts.sign(&request, token: account.token)
        return request
    }

    func container(_ path: String, _ query: [String: String] = [:]) async throws -> [String: Any] {
        let value: [String: Any]
        do { value = try await SourceHTTP.json(request(path, query)) }
        catch ClientError.unavailable { value = try await SourceHTTP.json(request(path, query, base: PlexRoutes.shared.resolve(account))) }
        guard let container = value["MediaContainer"] as? [String: Any] else { throw ClientError.invalidResponse }
        return container
    }

    func send(_ path: String, _ query: [String: String] = [:], method: String = "GET") async throws {
        do { _ = try await SourceHTTP.data(request(path, query, method: method), maximum: 1024 * 1024) }
        catch ClientError.unavailable { _ = try await SourceHTTP.data(request(path, query, method: method, base: PlexRoutes.shared.resolve(account)), maximum: 1024 * 1024) }
    }

    private func map(_ rows: Any?) -> [DeviceItem] {
        ((rows as? [[String: Any]]) ?? []).compactMap { PlexMapping.item($0, source: account.id) }
    }

    private func sections() async throws -> [(key: String, type: String)] {
        let value = try await container("/library/sections")
        return ((value["Directory"] as? [[String: Any]]) ?? []).prefix(64).compactMap { row in
            guard let key = SourceHTTP.identifier(row["key"]), let type = row["type"] as? String, ["movie", "show"].contains(type) else { return nil }
            return (key, type)
        }
    }

    private func paged(_ path: String, _ query: [String: String]) async throws -> [DeviceItem] {
        var collected: [DeviceItem] = []
        var start = 0
        while true {
            var page = query
            page["X-Plex-Container-Start"] = String(start)
            page["X-Plex-Container-Size"] = String(Self.pageSize)
            let value = try await container(path, page)
            let rows = (value["Metadata"] as? [[String: Any]]) ?? []
            collected += map(rows)
            start += rows.count
            let total = value["totalSize"] as? Int ?? start
            if rows.isEmpty || start >= total || collected.count >= 100_000 { return collected }
        }
    }

    func topLevel() async throws -> [DeviceItem] {
        var items: [DeviceItem] = []
        for section in try await sections() {
            items += try await paged("/library/sections/\(section.key)/all", ["includeGuids": "1"])
        }
        return items
    }

    func episodes(series: String) async throws -> [DeviceItem] {
        guard let id = SourceHTTP.identifier(series) else { throw DeviceFailure.notFound }
        return try await paged("/library/metadata/\(id)/allLeaves", ["includeGuids": "1"])
    }

    func item(_ id: String) async throws -> DeviceItem {
        try await metadata(id).item
    }

    func metadata(_ rawID: String, extras: [String: String] = [:]) async throws -> (raw: [String: Any], item: DeviceItem) {
        guard let id = SourceHTTP.identifier(rawID) else { throw DeviceFailure.notFound }
        let value: [String: Any]
        do { value = try await container("/library/metadata/\(id)", ["includeGuids": "1"].merging(extras) { $1 }) }
        catch ClientError.http(404) { throw DeviceFailure.notFound }
        guard let raw = (value["Metadata"] as? [[String: Any]])?.first, let item = PlexMapping.item(raw, source: account.id) else { throw DeviceFailure.notFound }
        return (raw, item)
    }

    func inProgress() async throws -> [DeviceItem] {
        var items: [DeviceItem] = []
        for section in try await sections() {
            let value = try await container("/library/sections/\(section.key)/all", [
                "type": section.type == "movie" ? "1" : "4", "sort": "lastViewedAt:desc", "includeGuids": "1",
                "X-Plex-Container-Start": "0", "X-Plex-Container-Size": "100"])
            items += map(value["Metadata"]).filter { $0.state.lastPlayed != nil || $0.state.seconds > 0 }
        }
        return items
    }

    func search(_ query: String) async throws -> [DeviceItem] {
        let value = try await container("/hubs/search", ["query": String(query.prefix(200)), "limit": "50", "includeCollections": "0"])
        return ((value["Hub"] as? [[String: Any]]) ?? []).filter { ["movie", "show", "episode"].contains($0["type"] as? String ?? "") }
            .flatMap { map($0["Metadata"]) }
    }

    func image(_ ref: SourceRef, kind: DeviceImageKind, width: Int) throws -> URLRequest {
        let source: String
        if kind == .person {
            guard ref.item.range(of: "^[0-9a-z]/people/[0-9a-f]{32}\\.(jpg|png)$", options: .regularExpression) != nil else { throw DeviceFailure.notFound }
            source = "https://metadata-static.plex.tv/" + ref.item
        } else {
            guard let id = SourceHTTP.identifier(ref.item), id.range(of: "^[0-9]+$", options: .regularExpression) != nil else { throw DeviceFailure.notFound }
            source = "/library/metadata/\(id)/" + (kind == .backdrop ? "art" : "thumb")
        }
        let wide = min(3840, max(120, width))
        let tall = kind == .backdrop ? wide * 9 / 16 : wide * 3 / 2
        return try request("/photo/:/transcode", ["url": source, "width": String(wide), "height": String(tall), "minSize": "1", "upscale": "1"])
    }

    func report(_ id: String, seconds: Double, watched: Bool, event: DeviceProgressEvent, session: String) async throws {
        guard let id = SourceHTTP.identifier(id) else { throw DeviceFailure.notFound }
        let state = event == .stop ? "stopped" : "playing"
        var request = try request("/:/timeline", ["ratingKey": id, "key": "/library/metadata/\(id)", "state": state,
                                                  "time": String(Int(max(0, seconds) * 1000)), "identifier": Self.library])
        request.setValue(session, forHTTPHeaderField: "X-Plex-Session-Identifier")
        _ = try await SourceHTTP.data(request, maximum: 1024 * 1024)
        if event == .stop { try? await send("/video/:/transcode/universal/stop", ["session": session]) }
        if watched || event == .stop { try await setPosition(id, seconds: seconds, watched: watched) }
    }

    func setPosition(_ id: String, seconds: Double, watched: Bool) async throws {
        guard let id = SourceHTTP.identifier(id) else { throw DeviceFailure.notFound }
        if watched { return try await send("/:/scrobble", ["key": id, "identifier": Self.library]) }
        if seconds <= 0 { try await send("/:/unscrobble", ["key": id, "identifier": Self.library]) }
        try await send("/:/progress", ["key": id, "identifier": Self.library, "time": String(Int(max(0, seconds) * 1000)), "state": "stopped"])
    }

    func setFavorite(_ id: String, favorite: Bool) async throws {
        throw DeviceFailure.unsupported
    }

    func playback(_ id: String, capabilities: DeviceCapabilities, start: Double) async throws -> DevicePlaybackInfo {
        try await PlexPlayback.info(client: self, id: id, capabilities: capabilities, start: start)
    }
}
