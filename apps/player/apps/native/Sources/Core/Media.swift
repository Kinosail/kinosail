import Foundation

struct WatchProgress: Codable, Hashable, Sendable {
    var seconds: Double = 0
    var watched = false
    var dismissed = false
    var session = ""
    var revision: Int = 0

    init() {}

    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["seconds", "watched", "session", "revision", "updated", "dismissed", "readerPage", "readerOffset"])
        seconds = try value.number("seconds")
        watched = try value.flag("watched", fallback: false)
        session = try value.text("session", max: 128)
        revision = Int(try value.number("revision", max: 9_007_199_254_740_991, integer: true))
        dismissed = try value.flag("dismissed", fallback: false)
        let page = try value.number("readerPage", max: 10_000_000, integer: true)
        let offset = try value.number("readerOffset", max: 1)
        guard page > 0 || offset == 0 else { throw ClientError.invalidResponse }
        if value["updated"] != nil { _ = try Input.date(value.text("updated", max: 40, required: true)) }
    }

    var json: JSONValue {
        .object(["seconds": .number(seconds), "watched": .bool(watched), "dismissed": .bool(dismissed),
                 "session": .string(session), "revision": .number(Double(revision))])
    }
}

enum MediaKind: String, Codable, Sendable {
    case video, show, music, audiobook, book, photo
    var symbol: String {
        switch self {
        case .video: "film"
        case .show: "tv"
        case .music: "music.note"
        case .audiobook: "headphones"
        case .book: "book"
        case .photo: "photo"
        }
    }
}

struct MediaItem: Codable, Identifiable, Hashable, Sendable {
    let id: String
    let kind: MediaKind
    let title: String
    let year: String
    let plot: String
    let rating: String
    let genres: String
    let artist: String
    let album: String
    let show: String
    let showID: String
    let season: Int
    let episode: Int
    let artwork: String
    let backdrop: String
    let container: String
    let stream: String
    let download: String
    let size: Int64
    var progress: WatchProgress
    // Optional for previously saved download metadata that predates cast support.
    let cast: [CastMember]?

    init(_ raw: JSONValue, server: ServerAddress) throws {
        var value = try raw.object(allowing: ["id", "kind", "title", "sortTitle", "year", "plot", "rating", "tagline", "genres", "director", "studio", "artist", "album", "track", "show", "showId", "size", "season", "episode", "stream", "download", "subtitles", "added", "cast", "artwork", "backdrop", "container", "progress"])
        // Some imported synopses contain zero-width word separators. Bound before normalizing;
        // other controls and every identifier/URL retain strict validation.
        if case .string(let plot)? = value["plot"], plot.utf8.count <= 10_000 {
            value["plot"] = .string(plot.replacingOccurrences(of: "\u{200B}", with: ""))
        }
        id = try Input.id(value.text("id", max: 128, required: true))
        let rawKind = try value.text("kind", max: 32, required: true)
        guard let kind = MediaKind(rawValue: rawKind == "audio" ? "music" : rawKind) else { throw ClientError.invalidResponse }
        self.kind = kind
        title = try value.text("title", max: 512, required: true)
        year = try value.text("year", max: 16)
        plot = try value.text("plot", max: 10_000)
        rating = try value.text("rating", max: 32)
        genres = try value.text("genres", max: 512)
        artist = try value.text("artist", max: 256)
        album = try value.text("album", max: 256)
        show = try value.text("show", max: 256)
        showID = try value.text("showId", max: 16)
        if !showID.isEmpty, showID.range(of: "^[a-f0-9]{16}$", options: .regularExpression) == nil { throw ClientError.invalidResponse }
        season = Int(try value.number("season", max: 100_000, integer: true))
        episode = Int(try value.number("episode", max: 100_000, integer: true))
        artwork = try value.text("artwork")
        backdrop = try value.text("backdrop")
        container = try value.text("container", max: 32)
        stream = try value.text("stream")
        download = try value.text("download")
        size = Int64(try value.number("size", max: 16_000_000_000_000, integer: true))
        progress = try WatchProgress(value["progress"] ?? .object([:]))
        for path in [artwork, backdrop, stream, download] where !path.isEmpty { _ = try server.mediaURL(path) }
        for key in ["sortTitle", "tagline", "director", "studio"] { _ = try value.text(key, max: 512) }
        for key in ["track", "subtitles"] { _ = try value.number(key, max: 100_000, integer: true) }
        if value["added"] != nil { _ = try Input.date(value.text("added", max: 40, required: true)) }
        cast = try CastMember.list(value, server: server)
    }

    var subtitle: String {
        if !show.isEmpty { return "" }
        return [artist, year, rating].filter { !$0.isEmpty }.joined(separator: " · ")
    }

    var subtitleWithoutYear: String {
        if !show.isEmpty { return "" }
        return [artist, rating].filter { !$0.isEmpty }.joined(separator: " · ")
    }

    var poster: String { show.isEmpty ? artwork : "/art/\(id)" }
    var isAudio: Bool { kind == .music || kind == .audiobook }
    var isUnwatched: Bool { (kind == .video || kind == .show) && !progress.watched }
    var playLabel: String {
        if kind == .photo { return "View photo" }
        if kind == .book { return "Read" }
        return progress.seconds > 0 && !progress.watched ? "Resume" : "Play"
    }

    var json: JSONValue {
        .object([
            "id": .string(id), "kind": .string(kind.rawValue), "title": .string(title), "year": .string(year),
            "plot": .string(plot), "rating": .string(rating), "genres": .string(genres), "artist": .string(artist),
            "album": .string(album), "show": .string(show), "showId": .string(showID), "season": .number(Double(season)),
            "episode": .number(Double(episode)), "artwork": .string(artwork), "backdrop": .string(backdrop),
            "container": .string(container), "stream": .string(stream), "download": .string(download),
            "size": .number(Double(size)), "progress": progress.json,
            "cast": .array((cast ?? []).map(\.json))
        ])
    }
}

enum LibraryView: String, CaseIterable, Identifiable, Sendable {
    case all, movies, shows, unwatched, list, music, audiobooks, books, photos, history
    var id: String { rawValue }
    var title: String {
        switch self {
        case .all: "All media"
        case .list: "My List"
        case .history: "History"
        case .books: "Ebooks & comics"
        default: rawValue.capitalized
        }
    }
}

enum LibrarySort: String, CaseIterable, Sendable {
    case title, added, year
    var title: String { self == .added ? "Recently added" : rawValue.capitalized }
}

struct LibraryPage: Sendable {
    struct Letter: Identifiable, Sendable {
        let label: String
        let offset: Int
        let count: Int
        var id: String { label }
    }
    let items: [MediaItem]
    let total: Int
    let offset: Int
    let limit: Int
    let letters: [Letter]

    init(_ raw: JSONValue, server: ServerAddress) throws {
        let value = try raw.object(allowing: ["items", "view", "sort", "query", "letter", "total", "offset", "limit", "letters"])
        total = Int(try value.requiredNumber("total", max: 10_000_000, integer: true))
        offset = Int(try value.requiredNumber("offset", max: 1_000_000, integer: true))
        limit = Int(try value.requiredNumber("limit", max: 200, integer: true))
        items = try value.required("items").array(max: 200).map { try MediaItem($0, server: server) }
        guard limit > 0, items.count <= limit, items.count <= max(0, total - offset), Set(items.map(\.id)).count == items.count else { throw ClientError.invalidResponse }
        if value["view"] != nil, try LibraryView(rawValue: value.text("view")) == nil { throw ClientError.invalidResponse }
        if value["sort"] != nil, try LibrarySort(rawValue: value.text("sort")) == nil { throw ClientError.invalidResponse }
        _ = try value.text("query", max: 512)
        _ = try value.text("letter", max: 32)
        var end = 0
        var parsed: [Letter] = []
        for rawLetter in try value.list("letters", max: 4096) {
            let entry = try rawLetter.object(allowing: ["label", "offset", "count"])
            let label = try entry.text("label", max: 32, required: true)
            let start = Int(try entry.requiredNumber("offset", max: 1_000_000, integer: true))
            let count = Int(try entry.requiredNumber("count", max: 10_000_000, integer: true))
            guard label.range(of: "\\A(?:#|\\p{L}[\\p{L}\\p{M}]{0,3})\\z", options: .regularExpression) != nil,
                  count > 0, start == end, start + count <= total else { throw ClientError.invalidResponse }
            parsed.append(Letter(label: label, offset: start, count: count))
            end = start + count
        }
        guard parsed.isEmpty || end == total else { throw ClientError.invalidResponse }
        letters = try Input.unique(parsed)
    }
}

struct Viewer: Codable, Sendable {
    let serverName: String
    let serverID: String
    let id: String
    let name: String
    let owner: Bool
    let downloads: Bool
    let transcode: Bool

    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["server", "serverId", "viewer", "sso", "language", "languagePreference", "languages"])
        let profile = try value.required("viewer").object(allowing: ["id", "name", "owner", "downloads", "transcode", "remote", "rating", "accessStart", "accessEnd", "libraries"])
        serverName = try value.text("server", max: 120, required: true)
        serverID = try value.text("serverId", max: 256, required: true)
        id = try Input.id(profile.text("id", max: 128, required: true))
        name = try profile.text("name", max: 120, required: true)
        owner = try profile.flag("owner")
        downloads = try profile.flag("downloads")
        transcode = try profile.flag("transcode")
        _ = try profile.flag("remote")
        for key in ["rating", "accessStart", "accessEnd"] { _ = try profile.text(key, max: 80) }
        for path in try profile.list("libraries", max: 256) {
            guard case .string(let text) = path, text.utf8.count <= 4096 else { throw ClientError.invalidResponse }
        }
        _ = try value.flag("sso", fallback: false)
        _ = try value.text("language", max: 80)
        _ = try value.text("languagePreference", max: 80)
    }

    var json: JSONValue {
        .object(["server": .string(serverName), "serverId": .string(serverID),
                 "viewer": .object(["id": .string(id), "name": .string(name), "owner": .bool(owner),
                                    "downloads": .bool(downloads), "transcode": .bool(transcode), "remote": .bool(false)])])
    }
}

struct DeviceApproval: Identifiable, Sendable {
    let code: String
    let device: String
    let expires: Date
    var id: String { code }

    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["code", "device", "expiresAt"])
        code = try Input.code(value.text("code", max: 6, required: true))
        device = try value.text("device", max: 80)
        expires = try Input.date(value.text("expiresAt", max: 40, required: true))
    }
}

struct ConnectChallenge: Sendable {
    let code: String
    let secret: String
    init(_ raw: JSONValue) throws {
        let value = try raw.object(allowing: ["code", "secret", "expiresAt"])
        code = try Input.code(value.text("code", max: 6, required: true))
        secret = try Input.secret(value.text("secret", max: 512, required: true))
    }
}
