#if os(iOS)
import Network
import Synchronization
import Testing
import UIKit
@testable import KinosailPlayer

final class PreferencesLoopbackFixture: @unchecked Sendable {
    struct State { var preferences = MediaPreferences(); var saves = 0; var preferenceReads = 0; var libraryReads = 0; var populated = false; var fails = false; var delay = 0.0; var artwork = Data(); var landscape = Data() }
    let state = Mutex(State())
    let listener: NWListener
    let server: ServerAddress
    let viewer: Viewer
    var saves: Int { state.withLock { $0.saves } }
    var preferences: MediaPreferences { state.withLock { $0.preferences } }
    var preferenceReads: Int { state.withLock { $0.preferenceReads } }
    var libraryReads: Int { state.withLock { $0.libraryReads } }
    var artwork: Data { get { state.withLock { $0.artwork } } set { state.withLock { $0.artwork = newValue } } }
    var landscape: Data { get { state.withLock { $0.landscape } } set { state.withLock { $0.landscape = newValue } } }
    var populated: Bool { get { state.withLock { $0.populated } } set { state.withLock { $0.populated = newValue } } }
    var fails: Bool { get { state.withLock { $0.fails } } set { state.withLock { $0.fails = newValue } } }
    var delay: Double { get { state.withLock { $0.delay } } set { state.withLock { $0.delay = newValue } } }
    static let showID = "0123456789abcdef"
    static var movie: JSONValue { .object(["id": .string("movie"), "kind": .string("video"), "title": .string("The Last Observatory"),
        "year": .string("2025"), "genres": .string("Science fiction · Adventure"), "rating": .string("PG-13"),
        "plot": .string("An astronomer follows a mysterious signal across a quiet, unfamiliar world."),
        "artwork": .string("/art/movie"), "backdrop": .string("/backdrop/movie"), "progress": .object(["seconds": .number(120)])]) }
    static var episode: JSONValue { .object(["id": .string("episode"), "kind": .string("video"), "title": .string("Beyond the blue horizon"),
        "show": .string("Ocean Stories"), "showId": .string(showID), "season": .number(1), "episode": .number(1),
        "artwork": .string("/art/episode"), "backdrop": .string("/backdrop/episode"), "progress": .object(["seconds": .number(120)])]) }
    static var music: JSONValue { .object(["id": .string("track"), "kind": .string("music"), "title": .string("A quieter place"),
        "album": .string("After the rain"), "artist": .string("Isla North"), "artwork": .string("/art/track")]) }

    init() async throws {
        let parameters = NWParameters.tcp
        parameters.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: .any)
        let listener = try NWListener(using: parameters)
        self.listener = listener
        viewer = try Viewer(.object(["server": .string("Isolated preview"), "serverId": .string(UUID().uuidString),
            "viewer": .object(["id": .string("preview"), "name": .string("Preview"), "owner": .bool(false),
                               "downloads": .bool(false), "transcode": .bool(false), "remote": .bool(false)])]))
        let ready = Mutex(false)
        listener.stateUpdateHandler = { if case .ready = $0 { ready.withLock { $0 = true } } }
        listener.newConnectionHandler = { $0.cancel() }
        listener.start(queue: .global())
        for _ in 0..<250 where !ready.withLock({ $0 }) { try await Task.sleep(for: .milliseconds(10)) }
        try #require(ready.withLock { $0 })
        let port = try #require(listener.port)
        server = try ServerAddress("http://127.0.0.1:\(port.rawValue)")
        listener.newConnectionHandler = { [weak self] connection in
            connection.start(queue: .global())
            self?.receive(connection, data: Data())
        }
    }
    private func receive(_ connection: NWConnection, data: Data) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 131_072) { [weak self] chunk, _, complete, error in
            guard let self, let chunk, error == nil, data.count + chunk.count <= 131_072 else { connection.cancel(); return }
            let next = data + chunk
            guard let boundary = next.range(of: Data("\r\n\r\n".utf8)),
                  let header = String(data: next[..<boundary.lowerBound], encoding: .utf8) else {
                if !complete { receive(connection, data: next) } else { connection.cancel() }
                return
            }
            let lines = header.components(separatedBy: "\r\n")
            let length = lines.first { $0.lowercased().hasPrefix("content-length:") }
                .flatMap { Int($0.split(separator: ":").last!.trimmingCharacters(in: .whitespaces)) } ?? 0
            guard next.count - boundary.upperBound >= length else { receive(connection, data: next); return }
            let request = lines[0].split(separator: " ")
            guard request.count == 3 else { connection.cancel(); return }
            let path = String(request[1]).components(separatedBy: "?")[0]
            if path.hasPrefix("/art/") || path.hasPrefix("/backdrop/") || path.hasPrefix("/media/") {
                let image = path.hasPrefix("/backdrop/") ? landscape : artwork
                let response = Data("HTTP/1.1 200 OK\r\nContent-Type: image/png\r\nContent-Length: \(image.count)\r\nConnection: close\r\n\r\n".utf8) + image
                connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
                return
            }
            var body = JSONValue.object([:])
            if path == "/read/book/asset/chapter.xhtml" {
                let html = Data("<html><head><title>A quiet morning</title></head><body><h1>A quiet morning</h1><p>The harbor was still as the first boat crossed the water. Beyond the rooftops, the mountains caught the early light.</p><p>She opened the map and found the place where their journey would begin.</p></body></html>".utf8)
                let response = Data("HTTP/1.1 200 OK\r\nContent-Type: application/xhtml+xml\r\nContent-Length: \(html.count)\r\nConnection: close\r\n\r\n".utf8) + html
                connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
                return
            }
            if path == "/api/v1/books/book/reader" { body = .object(["id": .string("book"), "title": .string("The harbor"), "type": .string("epub"), "pages": .array([.object(["number": .number(1), "title": .string("A quiet morning"), "url": .string("/read/book/asset/chapter.xhtml")])])]) }
            if path == "/api/v1/books/book/reader/progress" { body = .object(["page": .number(1), "total": .number(1), "offset": .number(0)]) }
            if path == "/api/v1/me" { body = viewer.json }
            if path.hasSuffix("/watch-progress") { body = .object(["seconds": .number(120), "duration": .number(600)]) }
            if path == "/api/v1/me/media-preferences" {
                if request[0] == "GET" { state.withLock { $0.preferenceReads += 1 } }
                if request[0] == "PUT", let edited = try? MediaPreferences(StrictJSON.decode(Data(next.suffix(length)))) {
                    state.withLock { $0.preferences = edited; $0.saves += 1 }
                }
                body = preferences.json
            }
            if path == "/api/v1/library" {
                state.withLock { $0.libraryReads += 1 }
                let query = URLComponents(string: String(request[1]))?.queryItems
                let limit = query?.first { $0.name == "limit" }.flatMap { Int($0.value ?? "") } ?? 60
                let view = query?.first { $0.name == "view" }?.value
                let items = !populated ? [] : view == "music" ? [Self.music] : view == "shows" ? [Self.episode] : [Self.movie, Self.episode]
                body = .object(["items": .array(items), "total": .number(Double(items.count)), "offset": .number(0), "limit": .number(Double(limit))])
            }
            if path == "/api/v1/items/photo" { body = .object(["item": .object(["id": .string("photo"), "kind": .string("photo"), "title": .string("Morning light"), "stream": .string("/media/photo")]), "listed": .bool(false), "profileId": .string(viewer.id)]) }
            if path == "/api/v1/items/movie" || path == "/api/v1/items/episode" || path == "/api/v1/items/track" {
                body = .object(["item": path.hasSuffix("track") ? Self.music : path.hasSuffix("episode") ? Self.episode : Self.movie,
                                "listed": .bool(true), "profileId": .string(viewer.id)])
            }
            if path == "/api/v1/shows/\(Self.showID)" {
                body = .object(["id": .string(Self.showID), "title": .string("Ocean Stories"), "backdrop": .string("/backdrop/episode"), "episodes": .array(populated ? [Self.episode] : []), "cast": .array([])])
            }
            if path == "/api/v1/albums" { body = .object(["albums": .array(populated ? [.object(["id": .string("album"), "title": .string("After the rain"), "artist": .string("Isla North"), "artwork": .string("/art/track")])] : [])]) }
            if path == "/api/v1/albums/album" { body = .object(["id": .string("album"), "title": .string("After the rain"), "artist": .string("Isla North"), "tracks": .array(populated ? [Self.music] : [])]) }
            if path == "/api/v1/collections" { body = .object(["collections": .array(populated ? [.string("Sunday discoveries"), .string("Family favorites")] : [])]) }
            if path.hasPrefix("/api/v1/collections/") { body = .object(["name": .string(String(path.split(separator: "/").last!).removingPercentEncoding!), "items": .array(populated ? [Self.movie] : [])]) }
            if path.hasSuffix("/bookmarks") { body = .object(["bookmarks": .array([])]) }
            if path == "/api/v1/supporter/collection" {
                body = .object(["display": .string("automatic"), "badges": .array(populated ? [.object(["edition": .string("monthly"), "family": .string("living-standard"), "rank": .number(1), "name": .string("Voyager"), "active": .bool(true), "archived": .bool(false)])] : [])])
            }
            guard let encoded = try? JSONEncoder().encode(body) else { connection.cancel(); return }
            let status = fails && path != "/api/v1/me" ? "503 Service Unavailable" : "200 OK"
            let response = Data("HTTP/1.1 \(status)\r\nContent-Type: application/json\r\nContent-Length: \(encoded.count)\r\nConnection: close\r\n\r\n".utf8) + encoded
            DispatchQueue.global().asyncAfter(deadline: .now() + delay) {
                connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
            }
        }
    }
}
#endif
