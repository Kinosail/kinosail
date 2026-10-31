import Foundation

struct DeviceLibraryRouter: DeviceRequestHandler {
    static let profileID = "device"
    static let serverID = "kinosail-device-library"
    let catalog: DeviceCatalog
    let state: DeviceLocalState
    let streams: DeviceStreamRegistry

    func handle(_ request: DeviceRequest) async -> DeviceResponse {
        let started = ContinuousClock.now
        defer {
            let elapsed = started.duration(to: .now).components
            deviceLibraryLog.debug("operation=device-library-route route=\(Self.pattern(request.path), privacy: .public) method=\(request.method, privacy: .public) ms=\(elapsed.seconds * 1000 + elapsed.attoseconds / 1_000_000_000_000_000, privacy: .public)")
        }
        do { return try await route(request) }
        catch is CancellationError { return .failure(ClientError.unavailable) }
        catch {
            if (error as? DeviceFailure) != .notFound {
                deviceLibraryLog.warning("operation=device-library-route route=\(Self.pattern(request.path), privacy: .public) outcome=failed failure_class=\(String(describing: error).prefix(48), privacy: .public)")
            }
            return .failure(error)
        }
    }

    static func pattern(_ path: String) -> String {
        path.split(separator: "/").prefix(4).map { $0.range(of: "^[a-f0-9]{16}$", options: .regularExpression) != nil ? "{id}" : String($0) }.joined(separator: "/")
    }

    private func route(_ request: DeviceRequest) async throws -> DeviceResponse {
        let parts = request.path.split(separator: "/").map(String.init)
        let method = request.method == "HEAD" ? "GET" : request.method
        switch (method, parts.count, parts.first) {
        case ("GET", 2, "art"): return try await artwork(parts[1], kind: request.query["variant"] == "episode" ? .thumb : .primary)
        case ("GET", 2, "backdrop"): return try await artwork(parts[1], kind: .backdrop)
        case ("GET", 3, "person"): return try await person(parts[1], index: parts[2])
        case ("GET", 2, "media"): return try await media(parts[1])
        case ("GET", 3, "hls"): return try stream(request.path, itemID: parts[1])
        case ("GET", 3, "subtitle"): return try await subtitle(parts[1], file: parts[2])
        default: break
        }
        guard parts.count >= 3, parts[0] == "api", parts[1] == "v1" else { throw DeviceFailure.notFound }
        let tail = Array(parts.dropFirst(2))
        switch (method, tail.count, tail.first) {
        case ("GET", 1, "me"): return .json(200, Self.viewer)
        case ("DELETE", 1, "session"): return .empty(204)
        case ("GET", 1, "library"): return try await library(request)
        case ("GET", 2, "me") where tail[1] == "media-preferences": return .json(200, await state.mediaPreferences())
        case ("PUT", 2, "me") where tail[1] == "media-preferences":
            let value = try Self.body(request) { $0 }
            return .json(200, try await Self.validating { try await state.saveMediaPreferences(value) })
        case ("GET", 2, "shows"): return try await show(tail[1])
        case ("PUT", 3, "shows") where tail[2] == "watched": return try await DeviceLibraryWatched(catalog: catalog).show(id(tail[1]), request: request)
        case ("GET", 2, "items"): return try await detail(tail[1])
        case (_, 3, "items"): return try await itemAction(method, id: tail[1], action: tail[2], request: request)
        case ("PUT", 4, "items") where tail[2] == "progress" && tail[3] == "sync":
            return try await DeviceLibraryPlayback(catalog: catalog, state: state, streams: streams).sync(id(tail[1]), request: request)
        case ("GET", 1, "collections"), ("GET", 1, "albums"), ("GET", 1, "actor"), ("GET", 1, "remote-players"):
            throw DeviceFailure.unsupported
        default: throw DeviceFailure.notFound
        }
    }

    static var viewer: JSONValue {
        .object(["server": .string("My libraries"), "serverId": .string(serverID),
                 "viewer": .object(["id": .string(profileID), "name": .string("You"), "owner": .bool(true),
                                    "downloads": .bool(false), "transcode": .bool(true), "remote": .bool(false)])])
    }

    static func body<T>(_ request: DeviceRequest, _ parse: (JSONValue) throws -> T) throws -> T {
        do { return try parse(request.json()) } catch { throw DeviceFailure.invalid }
    }

    static func validating<T>(_ work: () async throws -> T) async throws -> T {
        do { return try await work() } catch is ClientError { throw DeviceFailure.invalid }
    }

    private func id(_ raw: String) throws -> String {
        guard raw.range(of: "^[a-f0-9]{16}$", options: .regularExpression) != nil else { throw DeviceFailure.notFound }
        return raw
    }

    private func library(_ request: DeviceRequest) async throws -> DeviceResponse {
        let view = request.query["view"] ?? "all"
        let query = (request.query["q"] ?? "").trimmingCharacters(in: .whitespacesAndNewlines)
        var items: [DeviceItem]
        var sorted = false
        switch view {
        case "history":
            items = try await catalog.recentActivity()
            sorted = query.isEmpty
        case "music", "audiobooks", "books", "photos":
            items = []
        default:
            items = query.isEmpty ? try await catalog.library() : try await catalog.search(query)
        }
        if !query.isEmpty, view != "history" {
            let needle = query.folding(options: [.caseInsensitive, .diacriticInsensitive], locale: nil)
            items = items.filter { item in
                let title = item.title.folding(options: [.caseInsensitive, .diacriticInsensitive], locale: nil)
                return title.contains(needle) || item.seriesTitle.folding(options: [.caseInsensitive, .diacriticInsensitive], locale: nil).contains(needle)
            }
            items.sort { ($0.title.lowercased().hasPrefix(query.lowercased()) ? 0 : 1, $0.sortTitle) < ($1.title.lowercased().hasPrefix(query.lowercased()) ? 0 : 1, $1.sortTitle) }
            sorted = (request.query["sort"] ?? "title") == "title"
        }
        switch view {
        case "movies": items = items.filter { $0.kind == .movie }
        case "shows": items = items.filter { $0.kind == .series }
        case "unwatched": items = items.filter { !$0.state.watched && $0.kind != .episode }
        case "list": items = items.filter { $0.state.favorite }
        case "all": items = items.filter { $0.kind != .episode || !query.isEmpty }
        default: break
        }
        let marks = await state.marks(items.map(\.id))
        return .json(200, try DeviceProjection.page(items, request: request, sorted: sorted, marks: marks))
    }

    private func detail(_ raw: String) async throws -> DeviceResponse {
        let item = try await catalog.detail(id(raw))
        return .json(200, .object(["item": DeviceProjection.item(item, mark: await state.mark(item.id)),
                                   "listed": .bool(item.state.favorite), "profileId": .string(Self.profileID)]))
    }

    private func show(_ raw: String) async throws -> DeviceResponse {
        let (series, episodes) = try await catalog.show(id(raw))
        var value: [String: JSONValue] = ["id": .string(series.id), "title": .string(String(series.title.prefix(512)))]
        if series.hasBackdrop { value["backdrop"] = .string("/backdrop/\(series.id)") }
        if !series.year.isEmpty { value["year"] = .string(series.year) }
        if !series.plot.isEmpty { value["plot"] = .string(String(series.plot.prefix(10_000))) }
        if !series.genres.isEmpty { value["genres"] = .string(String(series.genres.prefix(512))) }
        if !series.studio.isEmpty { value["studio"] = .string(String(series.studio.prefix(512))) }
        var rows: [JSONValue] = []
        let marks = await state.marks(episodes.map(\.id))
        for episode in episodes { rows.append(DeviceProjection.item(episode, mark: marks[episode.id] ?? ProgressMark())) }
        value["episodes"] = .array(rows)
        return .json(200, .object(value))
    }

    private func itemAction(_ method: String, id raw: String, action: String, request: DeviceRequest) async throws -> DeviceResponse {
        let id = try id(raw)
        switch (method, action) {
        case ("GET", "playback"): return try await DeviceLibraryPlayback(catalog: catalog, state: state, streams: streams).playback(id, request: request)
        case ("GET", "watch-progress"):
            guard let item = try await catalog.lookup(id) else { throw DeviceFailure.notFound }
            return .json(200, .object(["seconds": .number(item.state.seconds), "duration": .number(max(0, item.duration))]))
        case ("GET", "playback-preferences"): return .json(200, await state.itemPreferences(id))
        case ("PUT", "playback-preferences"):
            let value = try Self.body(request) { $0 }
            return .json(200, try await Self.validating { try await state.saveItemPreferences(id, value) })
        case ("DELETE", "playback-preferences"): return .json(200, try await state.saveItemPreferences(id, nil))
        case ("PUT", "list"): return try await list(id, request: request)
        case ("PUT", "watched"): return try await DeviceLibraryWatched(catalog: catalog).item(id, request: request)
        case ("DELETE", "continue-watching"): return try await DeviceLibraryPlayback(catalog: catalog, state: state, streams: streams).dismiss(id)
        case ("POST", "playback-events"): return try await DeviceLibraryPlayback(catalog: catalog, state: state, streams: streams).event(id, request: request)
        default: throw DeviceFailure.unsupported
        }
    }

    private func list(_ id: String, request: DeviceRequest) async throws -> DeviceResponse {
        let listed = try Self.body(request) { try $0.object(allowing: ["listed"]).flag("listed") }
        guard let item = try await catalog.lookup(id) else { throw DeviceFailure.notFound }
        var stored = false
        for version in item.versions.isEmpty ? [item.ref] : item.versions {
            do { try await catalog.client(for: version).setFavorite(version.item, favorite: listed); stored = true }
            catch DeviceFailure.unsupported { continue }
        }
        guard stored else { throw DeviceFailure.unsupported }
        await catalog.updateState(id) { $0.favorite = listed }
        return .json(200, .object(["listed": .bool(listed)]))
    }

    private func artwork(_ raw: String, kind: DeviceImageKind) async throws -> DeviceResponse {
        guard let item = try await catalog.lookup(id(raw)) else { throw DeviceFailure.notFound }
        var target = item.ref
        var type = kind
        if item.kind == .episode {
            switch kind {
            case .thumb: type = .primary
            case .primary: target = item.seriesRef ?? item.ref
            case .backdrop: if !item.hasBackdrop, let series = item.seriesRef { target = series }
            case .person: break
            }
        } else if kind == .thumb { type = .primary }
        let request = try await catalog.client(for: target).image(target, kind: type, width: type == .backdrop ? 1920 : 800)
        return try await image(request)
    }

    private func person(_ raw: String, index: String) async throws -> DeviceResponse {
        let item = try await catalog.detail(id(raw))
        guard let position = Int(index), item.cast.indices.contains(position), let ref = item.cast[position].image else { throw DeviceFailure.notFound }
        return try await image(catalog.client(for: ref).image(ref, kind: .person, width: 400))
    }

    private func image(_ request: URLRequest) async throws -> DeviceResponse {
        let (data, response) = try await BoundedHTTPResponse.receive(request, session: SourceHTTP.session, maximum: 24 * 1024 * 1024, expected: [200])
        let type = response.mimeType?.lowercased() ?? ""
        guard ["image/jpeg", "image/png", "image/webp"].contains(type) else { throw DeviceFailure.notFound }
        return .data(200, data, type)
    }

    private func media(_ raw: String) async throws -> DeviceResponse {
        let id = try id(raw)
        guard let playback = await state.playback(id), let request = playback.direct else { throw DeviceFailure.notFound }
        return .proxy(request, nil)
    }

    private func stream(_ path: String, itemID: String) throws -> DeviceResponse {
        guard let request = streams.resolve(path, itemID: try id(itemID)), let url = request.url else { throw DeviceFailure.notFound }
        let rewrite = url.path.lowercased().hasSuffix(".m3u8")
            ? DeviceManifestRewrite(itemID: itemID, base: url) { [streams] next in try streams.register(next, itemID: itemID, template: request) }
            : nil
        return .proxy(request, rewrite)
    }

    private func subtitle(_ raw: String, file: String) async throws -> DeviceResponse {
        let id = try id(raw)
        guard file.hasSuffix(".vtt"), let index = Int(file.dropLast(4)), let playback = await state.playback(id),
              let request = playback.subtitles[index] else { throw DeviceFailure.notFound }
        let (data, _) = try await BoundedHTTPResponse.receive(request, session: SourceHTTP.session, maximum: 8 * 1024 * 1024, expected: [200])
        return .data(200, Self.webVTT(data), "text/vtt")
    }

    static func webVTT(_ data: Data) -> Data {
        guard var text = String(data: data, encoding: .utf8) ?? String(data: data, encoding: .isoLatin1) else { return data }
        if text.hasPrefix("\u{FEFF}") { text.removeFirst() }
        if text.hasPrefix("WEBVTT") { return data }
        let lines = text.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\r", with: "\n").components(separatedBy: "\n")
        let cues = lines.map { line in
            line.contains("-->") ? line.replacingOccurrences(of: #"(\d{2}:\d{2}:\d{2}),(\d{3})"#, with: "$1.$2", options: .regularExpression) : line
        }
        return Data(("WEBVTT\n\n" + cues.joined(separator: "\n")).utf8)
    }
}
