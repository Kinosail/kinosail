import Foundation
import Network
import Synchronization
import Testing
@testable import KinosailPlayer

// An isolated HTTP listener exercises the production client and playback
// coordinator. Only fictional identity and bounded JSON cross this interface.
final class PlaybackStartupFixture: @unchecked Sendable {
    struct State { var counts: [String: Int] = [:]; var delayed = false; var denied = true }
    let state = Mutex(State())
    let listener: NWListener
    let server: ServerAddress
    let client: ServerClient
    private let queue = DispatchQueue(label: "playback-startup-fixture")
    var delayed: Bool { get { state.withLock { $0.delayed } } set { state.withLock { $0.delayed = newValue } } }
    var denied: Bool { get { state.withLock { $0.denied } } set { state.withLock { $0.denied = newValue } } }
    func count(_ path: String) -> Int { state.withLock { $0.counts[path] ?? 0 } }

    init() async throws {
        let parameters = NWParameters.tcp
        parameters.requiredLocalEndpoint = .hostPort(host: "127.0.0.1", port: .any)
        listener = try NWListener(using: parameters)
        let ready = Mutex(false)
        listener.stateUpdateHandler = { if case .ready = $0 { ready.withLock { $0 = true } } }
        listener.newConnectionHandler = { $0.cancel() }
        listener.start(queue: queue)
        for _ in 0..<500 where !ready.withLock({ $0 }) { try await Task.sleep(for: .milliseconds(10)) }
        try #require(ready.withLock { $0 })
        server = try ServerAddress("http://127.0.0.1:\(try #require(listener.port).rawValue)")
        let viewer = try Viewer(.object(["server": .string("Isolated startup"), "serverId": .string("startup-fixture"),
            "viewer": .object(["id": .string("fixture"), "name": .string("Fixture"), "owner": .bool(false),
                               "downloads": .bool(false), "transcode": .bool(false), "remote": .bool(false)])]))
        client = try ServerClient(server: server, token: "fictional-startup-token", viewer: viewer)
        listener.newConnectionHandler = { [weak self] connection in
            guard let self else { connection.cancel(); return }
            connection.start(queue: self.queue)
            self.receive(connection, data: Data())
        }
    }
    func close() { listener.cancel() }
    func item(_ id: String) throws -> MediaItem {
        try MediaItem(.object(["id": .string(id), "kind": .string("video"), "title": .string("Fixture")]), server: server)
    }
    private func receive(_ connection: NWConnection, data: Data) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 8192) { [weak self] chunk, _, complete, error in
            guard let self, error == nil, let chunk, data.count + chunk.count <= 8192 else { connection.cancel(); return }
            let next = data + chunk
            guard let boundary = next.range(of: Data("\r\n\r\n".utf8)),
                  let header = String(data: next[..<boundary.lowerBound], encoding: .utf8) else {
                if complete { connection.cancel() } else { self.receive(connection, data: next) }
                return
            }
            let parts = header.components(separatedBy: "\r\n")[0].split(separator: " ")
            guard parts.count == 3, parts[0] == "GET", let url = URL(string: String(parts[1]), relativeTo: self.server.url) else { connection.cancel(); return }
            let path = url.path
            let (count, delayed, denied) = self.state.withLock { state in
                state.counts[path, default: 0] += 1
                return (state.counts[path]!, state.delayed, state.denied)
            }
            let id = path.split(separator: "/").dropFirst(3).first.map(String.init) ?? "movie"
            let preferences = path.hasSuffix("/playback-preferences")
            let body = preferences ? "{\"playback\":{},\"overridden\":false}" : "{\"media\":{\"duration\":60},\"plan\":{\"allowed\":true,\"mode\":\"direct\",\"reason\":\"direct-preferred\"},\"duration\":60,\"start\":0,\"directAllowed\":true,\"direct\":\"/media/\(id)\",\"directType\":\"video/mp4\"}"
            let payload = Data((denied ? "{\"error\":\"denied\"}" : body).utf8)
            let status = denied ? "403 Forbidden" : "200 OK"
            let response = Data("HTTP/1.1 \(status)\r\nContent-Type: application/json\r\nContent-Length: \(payload.count)\r\nConnection: close\r\n\r\n".utf8) + payload
            // A1's preference producer outlives its cancelled source reader;
            // A2 is still pending when A1 reports cancellation.
            let delay = !delayed ? 0.0 : preferences ? 0.5 : id == "a" && count > 1 ? 1.0 : 0.2
            self.queue.asyncAfter(deadline: .now() + delay) {
                connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
            }
        }
    }
}
