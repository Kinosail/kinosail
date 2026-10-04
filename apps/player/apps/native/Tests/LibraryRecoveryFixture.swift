#if os(iOS) || os(tvOS)
import Network
import Synchronization
import Testing
import UIKit
@testable import KinosailPlayer

/// Fictional loopback data. Responses are held until the native view has shown
/// its saved content, so authorization assertions do not depend on timing.
final class LibraryRecoveryFixture: @unchecked Sendable {
    struct State {
        var photoReads = 0
        var held = false
        var pending: [NWConnection] = []
        var libraryReads = 0
        var libraryHeld = false
        var libraryPending: [(NWConnection, String)] = []
        var nextReads = 0
    }
    let state = Mutex(State())
    let listener: NWListener
    let server: ServerAddress
    let viewer: Viewer
    var photoReads: Int { state.withLock { $0.photoReads } }
    var held: Bool { get { state.withLock { $0.held } } set { state.withLock { $0.held = newValue } } }
    var libraryReads: Int { state.withLock { $0.libraryReads } }
    var libraryHeld: Bool { get { state.withLock { $0.libraryHeld } } set { state.withLock { $0.libraryHeld = newValue } } }
    var nextReads: Int { state.withLock { $0.nextReads } }
    let image = UIGraphicsImageRenderer(size: CGSize(width: 120, height: 80)).pngData { context in
        UIColor.magenta.setFill(); context.fill(CGRect(x: 0, y: 0, width: 120, height: 80))
    }

    init(downloads: Bool = false) async throws {
        let parameters = NWParameters.tcp
        parameters.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: .any)
        let listener = try NWListener(using: parameters)
        self.listener = listener
        viewer = try Viewer(.object(["server": .string("Recovery fixture"), "serverId": .string(UUID().uuidString),
            "viewer": .object(["id": .string("recovery"), "name": .string("Recovery"), "owner": .bool(false),
                               "downloads": .bool(downloads), "transcode": .bool(false), "remote": .bool(false)])]))
        let ready = Mutex(false)
        listener.stateUpdateHandler = { if case .ready = $0 { ready.withLock { $0 = true } } }
        listener.newConnectionHandler = { $0.cancel() }
        listener.start(queue: .global())
        for _ in 0..<250 where !ready.withLock({ $0 }) { try await Task.sleep(for: .milliseconds(10)) }
        try #require(ready.withLock { $0 })
        server = try ServerAddress("http://127.0.0.1:\(try #require(listener.port).rawValue)")
        listener.newConnectionHandler = { [weak self] connection in
            connection.start(queue: .global())
            self?.receive(connection, data: Data())
        }
    }

    func respondPhoto(status: Int) {
        let connections = state.withLock { value in
            let pending = value.pending; value.pending = []; return pending
        }
        for connection in connections { send(connection, status: status, type: "application/json", data: photo) }
    }

    func close() {
        listener.cancel()
        state.withLock { value in
            value.pending.forEach { $0.cancel() }; value.pending = []
            value.libraryPending.forEach { $0.0.cancel() }; value.libraryPending = []
        }
    }

    func respondLibrary(status: Int) {
        let pending = state.withLock { value in
            let found = value.libraryPending; value.libraryPending = []; return found
        }
        for (connection, path) in pending { send(connection, status: status, type: "application/json", data: library(path)) }
    }

    private func library(_ path: String) -> Data {
        let query = URLComponents(string: path)?.queryItems ?? []
        func text(_ name: String, _ fallback: String) -> String { query.first { $0.name == name }?.value ?? fallback }
        let item = JSONValue.object(["id": .string("movie"), "kind": .string("video"), "title": .string("Fixture movie")])
        return try! JSONEncoder().encode(JSONValue.object(["items": .array([item]), "view": .string(text("view", "all")),
            "sort": .string(text("sort", "title")), "query": .string(text("q", "")), "total": .number(1),
            "offset": .number(Double(text("offset", "0"))!), "limit": .number(Double(text("limit", "60"))!), "letters": .array([])]))
    }

    private var photo: Data {
        try! JSONEncoder().encode(JSONValue.object([
            "item": .object(["id": .string("photo"), "kind": .string("photo"), "title": .string("Fixture photo"),
                             "stream": .string("/media/photo")]),
            "listed": .bool(false), "profileId": .string(viewer.id)]))
    }

    private func receive(_ connection: NWConnection, data: Data) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 8192) { [weak self] chunk, _, complete, error in
            guard let self, let chunk, error == nil, data.count + chunk.count <= 8192 else { connection.cancel(); return }
            let next = data + chunk
            guard let boundary = next.range(of: Data("\r\n\r\n".utf8)),
                  let header = String(data: next[..<boundary.lowerBound], encoding: .utf8) else {
                if !complete { receive(connection, data: next) } else { connection.cancel() }; return
            }
            let parts = header.components(separatedBy: "\r\n")[0].split(separator: " ")
            guard parts.count == 3, parts[0] == "GET" else { connection.cancel(); return }
            let path = String(parts[1]).components(separatedBy: "?")[0]
            if path == "/api/v1/items/photo" {
                let pending = state.withLock { value in
                    value.photoReads += 1
                    if value.held { value.pending.append(connection) }
                    return value.held
                }
                if !pending { send(connection, status: 200, type: "application/json", data: photo) }
            } else if path == "/api/v1/library" {
                let pending = state.withLock { value in
                    value.libraryReads += 1
                    if value.libraryHeld { value.libraryPending.append((connection, String(parts[1]))) }
                    return value.libraryHeld
                }
                if !pending { send(connection, status: 200, type: "application/json", data: library(String(parts[1]))) }
            } else if path == "/api/v1/items/next" {
                state.withLock { $0.nextReads += 1 }
                send(connection, status: 503, type: "application/json", data: Data("{}".utf8))
            } else if path == "/api/v1/items/movie/watch-progress" {
                send(connection, status: 200, type: "application/json", data: Data("{\"seconds\":0}".utf8))
            } else if path == "/media/photo" { send(connection, status: 200, type: "image/png", data: image) }
            else if path == "/api/v1/me" {
                send(connection, status: 200, type: "application/json", data: try! JSONEncoder().encode(viewer.json))
            } else { send(connection, status: 404, type: "application/json", data: Data("{}".utf8)) }
        }
    }

    private func send(_ connection: NWConnection, status: Int, type: String, data: Data) {
        let response = Data("HTTP/1.1 \(status) Fixture\r\nContent-Type: \(type)\r\nContent-Length: \(data.count)\r\nConnection: close\r\n\r\n".utf8) + data
        connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
    }
}
#endif
