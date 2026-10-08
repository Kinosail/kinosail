import Foundation
import ImageIO
import Testing
import UniformTypeIdentifiers
@testable import KinosailPlayer

struct ArtworkPrefetchTests {
    @Test func prefetchedArtworkSurvivesReopenWithoutNetwork() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true),
                               "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let fixture = try HTTPFixture(body: "{}", viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/art/next", width: 800, height: 400)
        let loader = ArtworkLoader()
        try await loader.prefetch(paths: ["/art/next", "/art/next"], client: fixture.client, dimension: 800)
        for _ in 0..<200 where fixture.requests.isEmpty { try await Task.sleep(for: .milliseconds(5)) }
        let warm = try await loader.image(path: "/art/next", client: fixture.client, dimension: 800)
        #expect(warm.width == 800)
        #expect(fixture.requests.count == 1)
        let server = await fixture.client.server
        await fixture.client.close()
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.hold = true; $0[fixture.host]?.routes = [:] }
        let reopened = try ServerClient(server: server, viewer: viewer,
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        let disk = try await ArtworkLoader().image(path: "/art/next", client: reopened, dimension: 400)
        #expect(disk.width == 400)
        #expect(fixture.requests.count == 1)
        await reopened.close()
    }

    @Test func prefetchLeavesCapacityForVisibleArtworkAndClearStopsQueuedWork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let data = try artworkImageData()
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/art/slow"] = .init(data: data, status: 200, headers: ["Content-Type": "image/png"], hold: true)
        }
        try installArtworkImage(fixture, path: "/art/visible")
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/api/v1/library"] = .init(
                data: Data("{\"items\":[],\"total\":0,\"offset\":0,\"limit\":60}".utf8), status: 200, headers: [:])
        }
        let loader = ArtworkLoader()
        try await loader.prefetch(paths: ["/art/slow", "/art/queued"], client: fixture.client, dimension: 800)
        let deadline = ContinuousClock.now.advanced(by: .seconds(10))
        while fixture.requests.isEmpty && ContinuousClock.now < deadline {
            try await Task.sleep(for: .milliseconds(5))
        }
        #expect(fixture.requests.count == 1)
        #expect(fixture.requests.first?.url?.path == "/art/slow")
        let catalog = try await fixture.client.library(view: .movies)
        #expect(catalog.items.isEmpty)
        let visible = try await loader.image(path: "/art/visible", client: fixture.client, dimension: 800)
        #expect(visible.width == 400)
        #expect(fixture.requests.count == 3)
        await loader.clear()
        try await Task.sleep(for: .milliseconds(50))
        #expect(!fixture.requests.contains { $0.url?.path == "/art/queued" })
        await fixture.client.close()
    }

    @Test func promotesPrefetchedArtworkWhenItBecomesVisible() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let stalled = FixtureURLProtocol.Entry(data: Data(), status: 200, headers: [:], hold: true)
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/art/held1"] = stalled
            $0[fixture.host]?.routes["/art/held2"] = stalled
        }
        try installArtworkImage(fixture, path: "/art/visible")
        let loader = ArtworkLoader()
        let held = [1, 2].map { index in
            Task { try await loader.image(path: "/art/held\(index)", client: fixture.client, background: true) }
        }
        for _ in 0..<200 where fixture.requests.count < 2 { try await Task.sleep(for: .milliseconds(5)) }
        try await loader.prefetch(paths: ["/art/visible"], client: fixture.client)
        try await Task.sleep(for: .milliseconds(50))
        #expect(fixture.requests.count == 2)
        let visible = Task { try await loader.image(path: "/art/visible", client: fixture.client, dimension: 800) }
        for _ in 0..<200 where !fixture.requests.contains(where: { $0.url?.path == "/art/visible" }) {
            try await Task.sleep(for: .milliseconds(5))
        }
        let started = fixture.requests.contains { $0.url?.path == "/art/visible" }
        #expect(started)
        if started { #expect(try await visible.value.width == 400) }
        else { visible.cancel() }
        await loader.clear()
        for task in held { await #expect(throws: CancellationError.self) { try await task.value } }
        await fixture.client.close()
    }

    @Test(arguments: [
        ["/art/good", ""], ["/art/good", "/api/v1/me"], ["/art/good", "/media/movie"],
        ["/art/good", "https://elsewhere.example/art/movie"], ["/art/good", "/art/movie?token=secret"],
        ["/art/good", "/art/../api/v1/me"], [String(repeating: "x", count: 16_385)],
        Array(repeating: "/art/good", count: 25)
    ])
    func rejectsEntireInvalidPrefetchBatchBeforeSideEffects(_ paths: [String]) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) {
            try await ArtworkLoader().prefetch(paths: paths, client: fixture.client, dimension: 800)
        }
        #expect(fixture.requests.isEmpty)
    }

    @Test func cancelledPrefetchAndInvalidDimensionDoNotStartWork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let loader = ArtworkLoader()
        await #expect(throws: ClientError.self) {
            try await loader.prefetch(paths: ["/art/movie"], client: fixture.client, dimension: 799)
        }
        let task = Task {
            withUnsafeCurrentTask { $0?.cancel() }
            try await loader.prefetch(paths: ["/art/movie"], client: fixture.client, dimension: 800)
        }
        await #expect(throws: CancellationError.self) { try await task.value }
        try await loader.prefetch(paths: [], client: fixture.client, dimension: 800)
        #expect(fixture.requests.isEmpty)
    }

}
