import Foundation
import Testing
@testable import KinosailPlayer

struct CatalogRevalidationTests {
    @Test func homeRetainsBothPagesWhenHistoryChangesDuringRecentFetch() async throws {
        let fixture = try Fixture()
        defer { fixture.remove() }
        let seed = Task { try await fixture.client.library(view: .history, limit: 24) }
        try await fixture.waitForRequests(1)
        fixture.respond(view: "history", version: 1)
        _ = try await seed.value

        let refresh = Task { try await fixture.client.home(policy: .reload) }
        try await fixture.waitForRequests(2)
        fixture.respond(view: "history", version: 2)
        // Wait for the changed first page to persist while recent is still in flight.
        for _ in 0..<200 {
            let page = try await fixture.client.library(view: .history, limit: 24, policy: .cached)
            if page.items.first?.title == "Movie 2" { break }
            try await Task.sleep(for: .milliseconds(5))
        }
        let updated = try await fixture.client.library(view: .history, limit: 24, policy: .cached)
        #expect(updated.items.first?.title == "Movie 2")
        fixture.respond(view: "all", version: 2)
        _ = try await refresh.value
        let cached = try await fixture.client.home(policy: .cached)
        #expect(cached.continueWatching.first?.title == "Movie 2")
        #expect(cached.recent.first?.title == "Movie 2")
        #expect(fixture.pending.isEmpty)
        await fixture.client.close()
    }

    @Test func navigationCancellationDoesNotAbandonSharedPersistence() async throws {
        let fixture = try Fixture()
        defer { fixture.remove() }
        let first = Task { try await fixture.client.library(view: .movies) }
        try await fixture.waitForRequests(1)
        first.cancel()
        fixture.respond(view: "movies", version: 1)
        await #expect(throws: CancellationError.self) { try await first.value }
        let saved = try await fixture.client.library(view: .movies, policy: .cached)
        #expect(saved.items.first?.title == "Movie 1")
        #expect(fixture.pending.isEmpty)
        await fixture.client.close()
    }

    @Test func homeShowsOnlyActiveContinueWatchingFromPlaybackHistory() async throws {
        let fixture = try Fixture()
        defer { fixture.remove() }
        let request = Task { try await fixture.client.home() }
        try await fixture.waitForRequests(2)
        fixture.respond(view: "history", version: 1, items: [
            Self.item("active", progress: ["seconds": 120]),
            Self.item("dismissed", progress: ["seconds": 240, "dismissed": true]),
            Self.item("watched", progress: ["seconds": 360, "watched": true]),
            Self.item("no-progress", progress: [:])
        ])
        fixture.respond(view: "all", version: 1)

        let home = try await request.value
        #expect(home.continueWatching.map(\.id) == ["active"])
        #expect(home.recent.count == 1)
        await fixture.client.close()
    }

    private static func item(_ id: String, progress: [String: Any]) -> [String: Any] {
        ["id": id, "kind": "video", "title": id, "stream": "/media/\(id)", "progress": progress]
    }

    @Test func invalidResponseNeverWarmsTheCache() async throws {
        let fixture = try Fixture()
        defer { fixture.remove() }
        let request = Task { try await fixture.client.library(view: .movies) }
        try await fixture.waitForRequests(1)
        fixture.respond(view: "movies", version: 1, invalid: true)
        await #expect(throws: ClientError.invalidResponse) { try await request.value }
        await #expect(throws: CatalogCacheMiss.self) { try await fixture.client.library(view: .movies, policy: .cached) }
        #expect(fixture.pending.isEmpty)
        await fixture.client.close()
    }

    private struct Fixture: Sendable {
        let client: ServerClient
        let host = UUID().uuidString.lowercased() + ".example.invalid"
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        var pending: [RevalidationProtocol] { RevalidationProtocol.pending.withLock { $0[host] ?? [] } }

        init() throws {
            let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
                "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true),
                                   "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
            client = try ServerClient(server: ServerAddress("https://\(host)"), viewer: viewer,
                                      protocolClasses: [RevalidationProtocol.self], cacheDirectory: directory)
        }

        func waitForRequests(_ count: Int) async throws {
            for _ in 0..<200 where pending.count < count { try await Task.sleep(for: .milliseconds(5)) }
            try #require(pending.count == count)
        }

        func respond(view: String, version: Int, invalid: Bool = false, items: [[String: Any]]? = nil) {
            let request = RevalidationProtocol.pending.withLock { requests -> RevalidationProtocol? in
                guard let index = requests[host]?.firstIndex(where: { $0.query["view"] == view }) else { return nil }
                return requests[host]?.remove(at: index)
            }
            #expect(request != nil)
            request?.respond(version: version, invalid: invalid, items: items)
        }

        func remove() {
            RevalidationProtocol.pending.withLock { _ = $0.removeValue(forKey: host) }
            try? FileManager.default.removeItem(at: directory)
        }
    }
}

private final class PendingRequests: @unchecked Sendable {
    private let lock = NSLock()
    private var storage: [String: [RevalidationProtocol]] = [:]

    func withLock<Result>(_ body: (inout [String: [RevalidationProtocol]]) throws -> Result) rethrows -> Result {
        try lock.withLock { try body(&storage) }
    }
}

private final class RevalidationProtocol: URLProtocol, @unchecked Sendable {
    static let pending = PendingRequests()
    var query: [String: String] {
        Dictionary(uniqueKeysWithValues: (URLComponents(url: request.url!, resolvingAgainstBaseURL: false)?.queryItems ?? [])
            .map { ($0.name, $0.value ?? "") })
    }
    override class func canInit(with request: URLRequest) -> Bool { true }
    override class func canonicalRequest(for request: URLRequest) -> URLRequest { request }
    override func startLoading() {
        Self.pending.withLock { $0[request.url!.host!, default: []].append(self) }
    }
    override func stopLoading() {
        Self.pending.withLock { $0[request.url!.host!]?.removeAll { $0 === self } }
    }
    func respond(version: Int, invalid: Bool, items: [[String: Any]]? = nil) {
        let query = query
        let item: [String: Any] = ["id": "movie", "kind": "video", "title": "Movie \(version)", "stream": "/media/movie", "progress": ["seconds": 120]]
        let responseItems = items ?? [item]
        let body: [String: Any] = ["items": responseItems, "view": invalid ? "wrong" : query["view"]!,
            "sort": query["sort"]!, "query": query["q"]!, "total": responseItems.count,
            "offset": Int(query["offset"]!)!, "limit": Int(query["limit"]!)!, "letters": []]
        let data = try! JSONSerialization.data(withJSONObject: body)
        let response = HTTPURLResponse(url: request.url!, statusCode: 200, httpVersion: nil,
                                       headerFields: ["Content-Type": "application/json"])!
        client?.urlProtocol(self, didReceive: response, cacheStoragePolicy: .notAllowed)
        client?.urlProtocol(self, didLoad: data)
        client?.urlProtocolDidFinishLoading(self)
    }
}
