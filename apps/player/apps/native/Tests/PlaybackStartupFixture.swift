import Foundation
import Network
import Synchronization
import Testing
@testable import KinosailPlayer

// An isolated HTTP listener exercises the production client and playback
// coordinator. Only fictional identity and bounded JSON cross this interface.
final class PlaybackStartupFixture: @unchecked Sendable {
    struct State { var counts: [String: Int] = [:]; var delayed = false; var denied = true; var media: Data?; var savedSeconds: Double? }
    let state = Mutex(State())
    let listener: NWListener
    let server: ServerAddress
    let client: ServerClient
    let viewer: Viewer
    private let queue = DispatchQueue(label: "playback-startup-fixture")
    var delayed: Bool { get { state.withLock { $0.delayed } } set { state.withLock { $0.delayed = newValue } } }
    var denied: Bool { get { state.withLock { $0.denied } } set { state.withLock { $0.denied = newValue } } }
    var media: Data? { get { state.withLock { $0.media } } set { state.withLock { $0.media = newValue } } }
    func count(_ path: String) -> Int { state.withLock { $0.counts[path] ?? 0 } }
    var savedSeconds: Double? { state.withLock { $0.savedSeconds } }

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
        viewer = try Viewer(.object(["server": .string("Isolated startup"), "serverId": .string("startup-fixture"),
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
    private func send(_ data: Data, type: String, connection: NWConnection) {
        let response = Data("HTTP/1.1 200 OK\r\nContent-Type: \(type)\r\nContent-Length: \(data.count)\r\nConnection: close\r\n\r\n".utf8) + data
        connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
    }
    private func sendMedia(_ media: Data, header: String, head: Bool, connection: NWConnection) {
        let range = header.components(separatedBy: "\r\n").first { $0.lowercased().hasPrefix("range:") }
            .map { $0.dropFirst(6).trimmingCharacters(in: .whitespaces) }
        var lower = 0, upper = media.count - 1
        if let range {
            guard (try? MediaRange.validate(range)) != nil else { connection.cancel(); return }
            let bounds = range.dropFirst(6).split(separator: "-", omittingEmptySubsequences: false)
            if bounds[0].isEmpty { lower = max(0, media.count - (Int(bounds[1]) ?? media.count)) }
            else { lower = Int(bounds[0]) ?? media.count; upper = min(upper, Int(bounds[1]) ?? upper) }
        }
        guard lower <= upper, upper < media.count else { connection.cancel(); return }
        let body = media.subdata(in: lower..<(upper + 1))
        let status = range == nil ? "200 OK" : "206 Partial Content"
        let contentRange = range == nil ? "" : "Content-Range: bytes \(lower)-\(upper)/\(media.count)\r\n"
        var response = Data("HTTP/1.1 \(status)\r\nContent-Type: video/mp4\r\nAccept-Ranges: bytes\r\nContent-Length: \(body.count)\r\n\(contentRange)Connection: close\r\n\r\n".utf8)
        if !head { response.append(body) }
        connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
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
            guard parts.count == 3, ["GET", "HEAD", "PUT"].contains(String(parts[0])), let url = URL(string: String(parts[1]), relativeTo: self.server.url) else { connection.cancel(); return }
            let path = url.path
            let length = header.components(separatedBy: "\r\n").first { $0.lowercased().hasPrefix("content-length:") }
                .flatMap { Int($0.split(separator: ":").last!.trimmingCharacters(in: .whitespaces)) } ?? 0
            guard (0...4096).contains(length) else { connection.cancel(); return }
            if next.count - boundary.upperBound < length {
                if complete { connection.cancel() } else { self.receive(connection, data: next) }
                return
            }
            let (count, delayed, denied) = self.state.withLock { state in
                state.counts[path, default: 0] += 1
                return (state.counts[path]!, state.delayed, state.denied)
            }
            if path.hasPrefix("/media/"), let media = self.media {
                self.sendMedia(media, header: header, head: parts[0] == "HEAD", connection: connection)
                return
            }
            if path == "/api/v1/me", !denied {
                self.send(try! JSONEncoder().encode(self.viewer.json), type: "application/json", connection: connection)
                return
            }
            if path.hasSuffix("/progress/sync") {
                guard let raw = try? StrictJSON.decode(Data(next[boundary.upperBound..<(boundary.upperBound + length)])),
                      let body = try? raw.object(allowing: ["progress", "expected", "playbackToken"]),
                      let progress = body["progress"], let data = try? JSONEncoder().encode(progress) else { connection.cancel(); return }
                if let saved = try? WatchProgress(progress) { self.state.withLock { $0.savedSeconds = saved.seconds } }
                self.send(data, type: "application/json", connection: connection)
                return
            }
            let id = path.split(separator: "/").dropFirst(3).first.map(String.init) ?? "movie"
            let preferences = path.hasSuffix("/playback-preferences")
            let preferencesBody = String(decoding: try! JSONEncoder().encode(JSONValue.object(["playback": PlaybackPreferences().json, "overridden": .bool(false)])), as: UTF8.self)
            let duration = self.media == nil ? 60 : 12
            let start = self.media == nil ? 0 : 3
            let body = preferences ? preferencesBody : "{\"media\":{\"duration\":\(duration)},\"plan\":{\"allowed\":true,\"mode\":\"direct\",\"reason\":\"direct-preferred\"},\"duration\":\(duration),\"start\":\(start),\"directAllowed\":true,\"direct\":\"/media/\(id)\",\"directType\":\"video/mp4\"}"
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
