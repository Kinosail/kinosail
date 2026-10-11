import Foundation
import Testing
@testable import KinosailPlayer

// These isolated checks cover disk migration, decoded memory and cancellation
// boundaries that a populated remote-focus journey cannot reliably measure.
struct ArtworkBackdropMigrationTests {
    @Test(arguments: [0, 429, 503])
    func migrationFailureUsesFreshLegacyBytesWithoutSuppressingRetry(_ status: Int) async throws {
        let fixture = try HTTPFixture(body: "{}", viewer: backdropMigrationViewer())
        defer { fixture.remove() }
        try await seedLegacyBackdrop(fixture)
        var failure = FixtureURLProtocol.Entry(data: Data(), status: status == 0 ? 200 : status, headers: [:])
        if status == 0 { failure.failure = .notConnectedToInternet }
        let response = failure
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.routes["/backdrop/movie"] = response }

        let loader = ArtworkLoader()
        let fallback = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        #expect(fallback.width == 780 && fallback.height == 439)
        let afterFailure = fixture.requests.count
        #expect(afterFailure > 1)
        if let store = try await fixture.client.cacheStore() { await store.flushWrites() }

        // The failed transition must not put legacy bytes into the new disk
        // namespace, nor retain them as fresh high-resolution memory pixels.
        try installArtworkImage(fixture, path: "/backdrop/movie", width: 3840, height: 2160)
        let upgraded = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        #expect(upgraded.width == 3840 && upgraded.height == 2160)
        #expect(fixture.requests.count == afterFailure + 1)
        await loader.clear()
        await fixture.client.close()
    }

    @Test(arguments: [401, 403, 404])
    func legacyBytesDoNotMaskAuthorizationOrMissingItem(_ status: Int) async throws {
        let fixture = try HTTPFixture(body: "{}", viewer: backdropMigrationViewer())
        defer { fixture.remove() }
        try await seedLegacyBackdrop(fixture)
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/backdrop/movie"] = .init(data: Data(), status: status, headers: [:])
        }
        let loader = ArtworkLoader()
        await #expect(throws: ClientError.http(status)) {
            try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        }
        #expect(fixture.requests.count == 2)
        await loader.clear()
        await fixture.client.close()
    }

    @Test func cancellationDoesNotReturnLegacyPixelsOrCompleteMigration() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: "{}", viewer: backdropMigrationViewer(), cacheDirectory: directory)
        defer { fixture.remove() }
        try await seedLegacyBackdrop(fixture)
        let data = try artworkImageData(width: 3840, height: 2160)
        ControlledArtworkProtocol.entries.withLock { $0[fixture.host] = .init(data: data) }
        defer { ControlledArtworkProtocol.entries.withLock { _ = $0.removeValue(forKey: fixture.host) } }
        let controlled = try ServerClient(server: await fixture.client.server, token: "fixture-token",
                                         viewer: backdropMigrationViewer(), protocolClasses: [ControlledArtworkProtocol.self],
                                         cacheDirectory: directory)
        let loader = ArtworkLoader()
        let pending = Task { try await loader.image(path: "/backdrop/movie", client: controlled, dimension: 4096) }
        for _ in 0..<200 where ControlledArtworkProtocol.requests(fixture.host) == 0 { try await Task.sleep(for: .milliseconds(5)) }
        #expect(ControlledArtworkProtocol.requests(fixture.host) == 1)
        pending.cancel()
        for _ in 0..<200 where !ControlledArtworkProtocol.active(fixture.host).isEmpty { try await Task.sleep(for: .milliseconds(5)) }
        let cancelledTransport = ControlledArtworkProtocol.active(fixture.host).isEmpty
        // Always release a broken cancellation path before awaiting its result.
        for response in ControlledArtworkProtocol.active(fixture.host) { response.finish() }
        #expect(cancelledTransport)
        await #expect(throws: CancellationError.self) { try await pending.value }
        await loader.clear()
        await controlled.close()
        try installArtworkImage(fixture, path: "/backdrop/movie", width: 3840, height: 2160)
        let upgraded = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        #expect(upgraded.width == 3840)
        #expect(fixture.requests.count == 2)
        await loader.clear()
        await fixture.client.close()
    }

    @Test(arguments: [false, true])
    func migrationFailureDoesNotUseStaleOrMalformedLegacyBytes(_ stale: Bool) async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: "{}", viewer: backdropMigrationViewer(), cacheDirectory: directory)
        defer { fixture.remove() }
        try await seedLegacyBackdrop(fixture)
        if stale {
            try ageArtwork(in: directory)
        } else if let store = try await fixture.client.cacheStore() {
            let url = try await fixture.client.server.mediaURL("/backdrop/movie")
            await store.enqueueWrite(Data("invalid image".utf8), key: url.absoluteString, kind: .artwork)
            await store.flushWrites()
        }
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/backdrop/movie"] = .init(data: Data(), status: 503, headers: [:])
        }
        let loader = ArtworkLoader()
        await #expect(throws: ClientError.http(503)) {
            try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        }
        #expect(fixture.requests.count > 1)
        await loader.clear()
        await fixture.client.close()
    }

    @Test func backdropLargerThanMemoryBudgetReusesPersistedBytes() async throws {
        let fixture = try HTTPFixture(body: "{}", viewer: backdropMigrationViewer())
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/backdrop/movie", width: 4096, height: 2304)
        let loader = ArtworkLoader()
        let first = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        #expect(first.width == 4096 && first.height == 2304)
        #expect(first.bytesPerRow * first.height > 32 * 1024 * 1024)
        if let store = try await fixture.client.cacheStore() { await store.flushWrites() }
        let repeated = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        #expect(repeated.width == 4096 && repeated.height == 2304)
        #expect(first !== repeated)
        #expect(fixture.requests.count == 1)
        await loader.clear()
        await fixture.client.close()
    }

    @Test func highResolutionDiskBytesRemainProfileScoped() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: "{}", viewer: backdropMigrationViewer(id: "first"), cacheDirectory: directory)
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/backdrop/movie", width: 3840, height: 2160)
        let loader = ArtworkLoader()
        _ = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        if let store = try await fixture.client.cacheStore() { await store.flushWrites() }

        let other = try ServerClient(server: await fixture.client.server, token: "second-profile-token",
                                     viewer: backdropMigrationViewer(id: "second"),
                                     protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        try installArtworkImage(fixture, path: "/backdrop/movie", width: 780, height: 439)
        let isolated = try await loader.image(path: "/backdrop/movie", client: other, dimension: 4096)
        #expect(isolated.width == 780)
        #expect(fixture.requests.count == 2)
        #expect(fixture.requests.last?.value(forHTTPHeaderField: "Authorization") == "Bearer second-profile-token")
        await loader.clear()
        await other.close()
        await fixture.client.close()
    }
}

private func backdropMigrationViewer(id: String = "viewer") throws -> Viewer {
    try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
        "viewer": .object(["id": .string(id), "name": .string("Viewer"), "owner": .bool(true),
                           "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
}

private func seedLegacyBackdrop(_ fixture: HTTPFixture) async throws {
    try installArtworkImage(fixture, path: "/backdrop/movie", width: 780, height: 439)
    let loader = ArtworkLoader()
    let legacy = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 800)
    #expect(legacy.width == 780)
    if let store = try await fixture.client.cacheStore() { await store.flushWrites() }
    await loader.clear()
}
