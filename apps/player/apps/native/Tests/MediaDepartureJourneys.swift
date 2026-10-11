import Foundation
import Network
import Synchronization
import Testing
@testable import KinosailPlayer

// Isolated HTTP adapter journey. Server departure E2E covers admission and
// shared viewers; it cannot detect a native gateway omitting the wire contract.
// Failures: absent/mismatched identity, replacement ending the new stream,
// duplicate departure, invalid input closing live playback, or blocked cleanup.
@Suite(.serialized)
struct MediaDepartureJourneys {
    @Test func failedDepartureDoesNotPreventAnotherStreamOpening() async throws {
        let fixture = try await MediaDepartureFixture()
        fixture.rejectDepartures = true
        defer { fixture.close() }
        let client = try ServerClient(server: fixture.server, token: "fictional-departure-token")
        let transport = MediaTransport()
        let reader = URLSession(configuration: .ephemeral)
        defer { reader.invalidateAndCancel() }
        let local = try await transport.open(url: fixture.server.mediaURL("/hls/first/master.m3u8"), itemID: "first", client: client)
        _ = try await reader.data(from: local)
        await transport.close()
        for _ in 0..<200 {
            if await client.reachability == .unreachable { break }
            try await Task.sleep(for: .milliseconds(10))
        }
        #expect(await client.reachability == .unreachable)
        #expect(fixture.departures.count == 1)
        let next = try await transport.open(url: fixture.server.mediaURL("/hls/next/master.m3u8"), itemID: "next", client: client)
        let (manifest, _) = try await reader.data(from: next)
        #expect(manifest.starts(with: Data("#EXTM3U".utf8)))
        await transport.close()
        for _ in 0..<200 where fixture.departures.count < 2 { try await Task.sleep(for: .milliseconds(10)) }
        await client.close()
    }

    @Test func nativeHLSClosesOnlyItsOwnSessionOnStopAndReplacement() async throws {
        let fixture = try await MediaDepartureFixture()
        defer { fixture.close() }
        let transport = MediaTransport(), other = MediaTransport()
        let client = try ServerClient(server: fixture.server, token: "fictional-departure-token")
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpCookieStorage = nil
        let reader = URLSession(configuration: configuration)
        defer { reader.invalidateAndCancel() }

        func read(_ transport: MediaTransport, _ id: String) async throws {
            let remote = try fixture.server.mediaURL("/hls/\(id)/master.m3u8")
            let local = try await transport.open(url: remote, itemID: id, client: client)
            let (manifest, _) = try await reader.data(from: local)
            let reference = try #require(String(data: manifest, encoding: .utf8)?.split(separator: "\n").first { !$0.hasPrefix("#") })
            let (segment, _) = try await reader.data(from: #require(URL(string: String(reference))))
            #expect(segment == Data("fixture segment".utf8))
        }

        try await read(transport, "first")
        try await read(other, "shared")
        do {
            _ = try await transport.open(url: fixture.server.mediaURL("/hls/wrong/master.m3u8"), itemID: "first", client: client)
            Issue.record("Invalid source was accepted")
        } catch { #expect(error is ClientError) }
        #expect(fixture.departures.isEmpty)
        try await read(transport, "next")
        await transport.close()
        await transport.close()
        for _ in 0..<200 where fixture.departures.count < 2 { try await Task.sleep(for: .milliseconds(10)) }
        #expect(fixture.departures.map(\.item).sorted() == ["first", "next"])
        await other.close()
        for _ in 0..<200 where fixture.departures.count < 3 { try await Task.sleep(for: .milliseconds(10)) }
        #expect(fixture.departures.count == 3)
        let sessions = fixture.mediaRequests.map(\.session)
        #expect(sessions.count == 6)
        #expect(sessions.allSatisfy { UUID(uuidString: $0) != nil })
        #expect(Set(sessions).count == 3)
        for departure in fixture.departures {
            let reads = fixture.mediaRequests.filter { $0.item == departure.item }
            #expect(reads.count == 2)
            #expect(reads.allSatisfy { $0.session == departure.session })
        }
        print("NATIVE HLS observed reads=\(sessions.count) departures=\(fixture.departures.count) identities=\(Set(sessions).count)")
        await client.close()
    }
}

private final class MediaDepartureFixture: @unchecked Sendable {
    struct Observation: Sendable { let item: String; let session: String }
    struct State { var reads: [Observation] = []; var ends: [Observation] = []; var rejectDepartures = false }
    private let state = Mutex(State())
    private let queue = DispatchQueue(label: "native-departure-fixture")
    private let listener: NWListener
    let server: ServerAddress
    var mediaRequests: [Observation] { state.withLock { $0.reads } }
    var departures: [Observation] { state.withLock { $0.ends } }
    var rejectDepartures: Bool { get { state.withLock { $0.rejectDepartures } } set { state.withLock { $0.rejectDepartures = newValue } } }

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
        listener.newConnectionHandler = { [weak self] connection in
            guard let self else { connection.cancel(); return }
            connection.start(queue: self.queue)
            self.receive(connection, data: Data())
        }
    }
    func close() { listener.cancel() }

    private func receive(_ connection: NWConnection, data: Data) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 8192) { [weak self] chunk, _, complete, error in
            guard let self, error == nil, let chunk, data.count + chunk.count <= 8192 else { connection.cancel(); return }
            let next = data + chunk
            guard let boundary = next.range(of: Data("\r\n\r\n".utf8)),
                  let header = String(data: next[..<boundary.lowerBound], encoding: .utf8) else {
                if complete { connection.cancel() } else { self.receive(connection, data: next) }
                return
            }
            let fields = header.components(separatedBy: "\r\n")
            let request = fields[0].split(separator: " ")
            guard request.count == 3, let url = URL(string: String(request[1])) else { connection.cancel(); return }
            func field(_ name: String) -> String {
                fields.first { $0.lowercased().hasPrefix(name + ":") }?.dropFirst(name.count + 1).trimmingCharacters(in: .whitespaces) ?? ""
            }
            let length = Int(field("content-length")) ?? 0
            guard (0...4096).contains(length) else { connection.cancel(); return }
            if next.count - boundary.upperBound < length {
                if complete { connection.cancel() } else { self.receive(connection, data: next) }
                return
            }
            let parts = url.path.split(separator: "/")
            let body: Data, status: String, type: String
            if request[0] == "GET", parts.count == 3, parts[0] == "hls" {
                self.state.withLock { $0.reads.append(Observation(item: String(parts[1]), session: field("x-playback-session"))) }
                body = Data((parts[2] == "master.m3u8" ? "#EXTM3U\n#EXTINF:1,\nsegment.ts\n#EXT-X-ENDLIST\n" : "fixture segment").utf8)
                status = "200 OK"; type = parts[2] == "master.m3u8" ? "application/vnd.apple.mpegurl" : "video/mp2t"
            } else if request[0] == "POST", parts.count == 5, parts[4] == "playback-events",
                      let raw = try? StrictJSON.decode(Data(next[boundary.upperBound..<(boundary.upperBound + length)])),
                      let object = try? raw.object(allowing: ["session", "event", "sequence"]),
                      case .string(let session) = object["session"], object["event"] == .string("session-end"),
                      object["sequence"] == .number(1) {
                self.state.withLock { $0.ends.append(Observation(item: String(parts[3]), session: session)) }
                body = Data(); status = self.rejectDepartures ? "503 Unavailable" : "204 No Content"; type = "application/json"
            } else {
                body = Data(); status = "400 Bad Request"; type = "application/json"
            }
            let response = Data("HTTP/1.1 \(status)\r\nContent-Type: \(type)\r\nContent-Length: \(body.count)\r\nConnection: close\r\n\r\n".utf8) + body
            connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
        }
    }
}
