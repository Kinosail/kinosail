import Foundation

struct JellyfinClient: MediaSourceClient {
    let account: SourceAccount
    private static let fields = "ProviderIds,Overview,Genres,DateCreated,OfficialRating,Studios,Taglines,SortName,PremiereDate,ProductionYear,RunTimeTicks,UserData,ImageTags,BackdropImageTags,ParentBackdropImageTags,SeriesPrimaryImageTag,Container,Width"
    private static let pageSize = 400

    static func authorization(token: String?) -> String {
        var value = "MediaBrowser Client=\"Kinosail\", Device=\"\(SourceHTTP.deviceName)\", DeviceId=\"\(SourceHTTP.installID)\", Version=\"\(SourceHTTP.version)\""
        if let token { value += ", Token=\"\(token)\"" }
        return value
    }

    static func request(_ url: URL, token: String?, method: String = "GET", body: [String: Any]? = nil) throws -> URLRequest {
        var request = URLRequest(url: url)
        request.httpMethod = method
        request.httpShouldHandleCookies = false
        let header = authorization(token: token)
        request.setValue(header, forHTTPHeaderField: "Authorization")
        request.setValue(header, forHTTPHeaderField: "X-Emby-Authorization")
        if let token { request.setValue(token, forHTTPHeaderField: "X-Emby-Token") }
        request.setValue("application/json", forHTTPHeaderField: "Accept")
        if let body {
            request.httpBody = try JSONSerialization.data(withJSONObject: body)
            request.setValue("application/json", forHTTPHeaderField: "Content-Type")
        }
        return request
    }

    func request(_ path: String, _ query: [String: String] = [:], method: String = "GET", body: [String: Any]? = nil) throws -> URLRequest {
        try Self.request(SourceHTTP.url(account.address, path, query), token: account.token, method: method, body: body)
    }

    private var user: String { account.userID }

    private func items(_ path: String, _ query: [String: String]) async throws -> [[String: Any]] {
        var collected: [[String: Any]] = []
        var start = 0
        while true {
            var page = query
            page["StartIndex"] = String(start)
            page["Limit"] = String(Self.pageSize)
            page["Fields"] = Self.fields
            let value = try await SourceHTTP.json(request(path, page))
            let batch = value["Items"] as? [[String: Any]] ?? []
            collected += batch
            let total = (value["TotalRecordCount"] as? Int) ?? collected.count
            start += batch.count
            if batch.isEmpty || start >= total || collected.count >= 100_000 { return collected }
        }
    }

    func topLevel() async throws -> [DeviceItem] {
        try await items("/Users/\(user)/Items", ["Recursive": "true", "IncludeItemTypes": "Movie,Series", "SortBy": "SortName"])
            .compactMap { JellyfinMapping.item($0, source: account.id) }
    }

    func episodes(series: String) async throws -> [DeviceItem] {
        try await items("/Shows/\(SourceHTTP.identifier(series) ?? "")/Episodes", ["UserId": user])
            .compactMap { JellyfinMapping.item($0, source: account.id) }
    }

    func item(_ id: String) async throws -> DeviceItem {
        guard let id = SourceHTTP.identifier(id) else { throw DeviceFailure.notFound }
        var query = ["Fields": Self.fields + ",People,MediaSources"]
        query["UserId"] = user
        let value = try await SourceHTTP.json(request("/Users/\(user)/Items/\(id)", query))
        guard let item = JellyfinMapping.item(value, source: account.id) else { throw DeviceFailure.notFound }
        return item
    }

    func inProgress() async throws -> [DeviceItem] {
        let resume = try await SourceHTTP.json(request("/Users/\(user)/Items/Resume", ["MediaTypes": "Video", "Limit": "100", "Fields": Self.fields]))
        let played = try await SourceHTTP.json(request("/Users/\(user)/Items", [
            "Recursive": "true", "IncludeItemTypes": "Movie,Episode", "SortBy": "DatePlayed", "SortOrder": "Descending",
            "Filters": "IsPlayed", "Limit": "100", "Fields": Self.fields]))
        let rows = (resume["Items"] as? [[String: Any]] ?? []) + (played["Items"] as? [[String: Any]] ?? [])
        return rows.compactMap { JellyfinMapping.item($0, source: account.id) }
    }

    func search(_ query: String) async throws -> [DeviceItem] {
        let value = try await SourceHTTP.json(request("/Users/\(user)/Items", [
            "Recursive": "true", "IncludeItemTypes": "Movie,Series,Episode", "SearchTerm": query, "Limit": "100", "Fields": Self.fields]))
        return (value["Items"] as? [[String: Any]] ?? []).compactMap { JellyfinMapping.item($0, source: account.id) }
    }

    func image(_ ref: SourceRef, kind: DeviceImageKind, width: Int) throws -> URLRequest {
        guard let id = SourceHTTP.identifier(ref.item) else { throw DeviceFailure.notFound }
        let type = switch kind {
        case .primary, .person: "Primary"
        case .backdrop: "Backdrop/0"
        case .thumb: "Thumb"
        }
        return try request("/Items/\(id)/Images/\(type)", ["maxWidth": String(min(3840, max(120, width))), "quality": "90"])
    }

    func report(_ id: String, seconds: Double, watched: Bool, event: DeviceProgressEvent, session: String) async throws {
        guard let id = SourceHTTP.identifier(id) else { throw DeviceFailure.notFound }
        let ticks = Int64(max(0, seconds) * 10_000_000)
        let body: [String: Any] = ["ItemId": id, "PositionTicks": ticks, "PlaySessionId": session, "CanSeek": true, "PlayMethod": "DirectPlay"]
        let path = switch event {
        case .start: "/Sessions/Playing"
        case .progress: "/Sessions/Playing/Progress"
        case .stop: "/Sessions/Playing/Stopped"
        }
        _ = try await SourceHTTP.data(request(path, method: "POST", body: body))
        if watched || event == .stop { try await setPosition(id, seconds: seconds, watched: watched) }
    }

    func setPosition(_ id: String, seconds: Double, watched: Bool) async throws {
        guard let id = SourceHTTP.identifier(id) else { throw DeviceFailure.notFound }
        if watched {
            _ = try await SourceHTTP.data(request("/Users/\(user)/PlayedItems/\(id)", method: "POST", body: [:]))
        } else {
            _ = try await SourceHTTP.data(request("/Users/\(user)/Items/\(id)/UserData", method: "POST",
                                                  body: ["PlaybackPositionTicks": Int64(max(0, seconds) * 10_000_000), "Played": false]))
        }
    }

    func setFavorite(_ id: String, favorite: Bool) async throws {
        guard let id = SourceHTTP.identifier(id) else { throw DeviceFailure.notFound }
        _ = try await SourceHTTP.data(request("/Users/\(user)/FavoriteItems/\(id)", method: favorite ? "POST" : "DELETE", body: favorite ? [:] : nil))
    }

    func playback(_ id: String, capabilities: DeviceCapabilities, start: Double) async throws -> DevicePlaybackInfo {
        try await JellyfinPlayback.info(client: self, id: id, capabilities: capabilities, start: start)
    }
}
