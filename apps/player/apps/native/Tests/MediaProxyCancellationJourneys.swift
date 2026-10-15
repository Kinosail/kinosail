import Foundation
import Network
import Synchronization
import Testing
@testable import KinosailPlayer

@Suite(.serialized)
struct MediaProxyCancellationJourneys {
    static let parallel = 8

    @Test func abandonedSegmentsReleaseUpstreamRequestsAndParallelStreamsAreServed() async throws {
        let fixture = try await HeldSegmentFixture()
        defer { fixture.close() }
        let client = try ServerClient(server: fixture.server, token: "fictional-proxy-token")
        let transport = MediaTransport()
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpMaximumConnectionsPerHost = Self.parallel * 2
        configuration.timeoutIntervalForRequest = 20
        let reader = URLSession(configuration: configuration)
        defer { reader.invalidateAndCancel() }

        let local = try await transport.open(url: fixture.server.mediaURL("/hls/movie/master.m3u8"), itemID: "movie", client: client)
        let (manifest, _) = try await reader.data(from: local)
        let references = try #require(String(data: manifest, encoding: .utf8)).split(separator: "\n").filter { !$0.hasPrefix("#") }.map(String.init)
        try #require(references.count == Self.parallel + 1)
        let held = try references.dropLast().map { try #require(URL(string: $0)) }
        let last = try #require(references.last)
        let ready = try #require(URL(string: last))

        let tasks = held.map { url in reader.dataTask(with: url) }
        tasks.forEach { $0.resume() }
        let opened = try await waitUntil(seconds: 5) { fixture.heldOpened == Self.parallel }
        print("MEDIA_PROXY parallelHeldUpstream=\(fixture.heldOpened) expected=\(Self.parallel)")
        #expect(opened, "Every parallel segment request must reach the Server instead of being reset by the proxy")

        tasks.forEach { $0.cancel() }
        let released = try await waitUntil(seconds: 5) { fixture.heldClosed == fixture.heldOpened && fixture.heldOpened > 0 }
        print("MEDIA_PROXY abandonedUpstreamClosed=\(fixture.heldClosed) opened=\(fixture.heldOpened)")
        #expect(released, "Abandoned player requests must cancel their upstream Server requests")

        let start = ContinuousClock.now
        var request = URLRequest(url: ready)
        request.timeoutInterval = 5
        let (segment, response) = try await reader.data(for: request)
        print("MEDIA_PROXY readyAfterAbandon status=\((response as? HTTPURLResponse)?.statusCode ?? 0) elapsed=\(ContinuousClock.now - start)")
        #expect((response as? HTTPURLResponse)?.statusCode == 200)
        #expect(segment == Data("fixture ready segment".utf8))

        await transport.close()
        await client.close()
    }

    @Test func proxyStillBoundsConcurrentPlayerConnections() async throws {
        let fixture = try await HeldSegmentFixture()
        defer { fixture.close() }
        let client = try ServerClient(server: fixture.server, token: "fictional-proxy-token")
        let transport = MediaTransport()
        let configuration = URLSessionConfiguration.ephemeral
        configuration.httpMaximumConnectionsPerHost = 64
        let reader = URLSession(configuration: configuration)
        defer { reader.invalidateAndCancel() }
        let local = try await transport.open(url: fixture.server.mediaURL("/hls/movie/master.m3u8"), itemID: "movie", client: client)
        let (manifest, _) = try await reader.data(from: local)
        let first = try #require(String(data: manifest, encoding: .utf8)?.split(separator: "\n").first { !$0.hasPrefix("#") })
        let url = try #require(URL(string: String(first)))
        let tasks = (0..<40).map { _ in reader.dataTask(with: url) }
        tasks.forEach { $0.resume() }
        _ = try await waitUntil(seconds: 3) { fixture.heldOpened >= 16 }
        try await Task.sleep(for: .milliseconds(500))
        print("MEDIA_PROXY boundedUpstream=\(fixture.heldOpened) attempted=40")
        #expect(fixture.heldOpened <= 16, "The loopback proxy must keep a fixed upper bound on upstream requests")
        tasks.forEach { $0.cancel() }
        await transport.close()
        await client.close()
    }

    private func waitUntil(seconds: Int, _ condition: () -> Bool) async throws -> Bool {
        let deadline = ContinuousClock.now.advanced(by: .seconds(seconds))
        while ContinuousClock.now < deadline {
            if condition() { return true }
            try await Task.sleep(for: .milliseconds(20))
        }
        return condition()
    }
}

private final class HeldSegmentFixture: @unchecked Sendable {
    struct State { var opened = 0; var closed = 0; var held: [ObjectIdentifier: NWConnection] = [:] }
    private let state = Mutex(State())
    private let queue = DispatchQueue(label: "native-held-segment-fixture")
    private let listener: NWListener
    let server: ServerAddress
    var heldOpened: Int { state.withLock { $0.opened } }
    var heldClosed: Int { state.withLock { $0.closed } }

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

    func close() {
        listener.cancel()
        let held = state.withLock { state in let values = Array(state.held.values); state.held.removeAll(); return values }
        held.forEach { $0.cancel() }
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
            let request = header.components(separatedBy: "\r\n")[0].split(separator: " ")
            guard request.count == 3, request[0] == "GET", let url = URL(string: String(request[1])) else { connection.cancel(); return }
            let name = url.lastPathComponent
            if name.hasPrefix("held-") {
                self.hold(connection)
                return
            }
            let body: Data, type: String
            if name == "master.m3u8" {
                let segments = (0..<MediaProxyCancellationJourneys.parallel).map { "#EXTINF:6,\nheld-\($0).ts\n" }.joined()
                body = Data("#EXTM3U\n#EXT-X-TARGETDURATION:6\n\(segments)#EXTINF:6,\nready.ts\n#EXT-X-ENDLIST\n".utf8)
                type = "application/vnd.apple.mpegurl"
            } else if name == "ready.ts" {
                body = Data("fixture ready segment".utf8); type = "video/mp2t"
            } else { connection.cancel(); return }
            let response = Data("HTTP/1.1 200 OK\r\nContent-Type: \(type)\r\nContent-Length: \(body.count)\r\nConnection: close\r\n\r\n".utf8) + body
            connection.send(content: response, completion: .contentProcessed { _ in connection.cancel() })
        }
    }

    private func hold(_ connection: NWConnection) {
        let identity = ObjectIdentifier(connection)
        state.withLock { $0.opened += 1; $0.held[identity] = connection }
        connection.stateUpdateHandler = { [weak self] state in
            switch state {
            case .failed, .cancelled: self?.release(connection)
            default: break
            }
        }
        watch(connection)
    }

    private func watch(_ connection: NWConnection) {
        connection.receive(minimumIncompleteLength: 1, maximumLength: 1024) { [weak self] data, _, complete, error in
            if complete || error != nil { self?.release(connection) } else if data != nil { self?.watch(connection) }
        }
    }

    private func release(_ connection: NWConnection) {
        let identity = ObjectIdentifier(connection)
        let removed = state.withLock { state in
            guard state.held.removeValue(forKey: identity) != nil else { return false }
            state.closed += 1; return true
        }
        if removed { connection.cancel() }
    }
}
