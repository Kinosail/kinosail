import Foundation
import ImageIO
import Testing
import UniformTypeIdentifiers
@testable import KinosailPlayer

struct ArtworkLoaderTests {
    @Test func abandonedPosterLoadsFreeCapacityForVisibleArtwork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let stalled = FixtureURLProtocol.Entry(data: Data(), status: 200, headers: [:], hold: true)
        FixtureURLProtocol.entries.withLock { entries in
            for index in 0..<4 { entries[fixture.host]?.routes["/art/old\(index)"] = stalled }
        }
        try installArtworkImage(fixture, path: "/art/visible")
        let loader = ArtworkLoader()
        let old = (0..<4).map { index in Task { try await loader.image(path: "/art/old\(index)", client: fixture.client) } }
        for _ in 0..<1_000 where fixture.requests.count < 4 { try await Task.sleep(for: .milliseconds(5)) }
        #expect(fixture.requests.count == 4)
        old.forEach { $0.cancel() }
        let visible = Task { try await loader.image(path: "/art/visible", client: fixture.client) }
        for _ in 0..<1_000 where !fixture.requests.contains(where: { $0.url?.path == "/art/visible" }) {
            try await Task.sleep(for: .milliseconds(5))
        }
        let started = fixture.requests.contains { $0.url?.path == "/art/visible" }
        #expect(started)
        if started { #expect(try await visible.value.width == 400) }
        else { visible.cancel() }
        await loader.clear()
        for task in old { await #expect(throws: CancellationError.self) { try await task.value } }
    }

    @Test func diskHitsRetainDecodedPixelsIncludingTopShelfSize() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true), "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let fixture = try HTTPFixture(body: "{}", viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/art/movie", width: 800, height: 400)
        _ = try await ArtworkLoader().image(path: "/art/movie", client: fixture.client, dimension: 400)
        if let store = try await fixture.client.cacheStore() { await store.flushWrites() }
        let loader = ArtworkLoader()
        let disk = try await loader.image(path: "/art/movie", client: fixture.client, dimension: 400)
        let memory = try await loader.image(path: "/art/movie", client: fixture.client, dimension: 400)
        #expect(disk === memory)
        #expect(disk.width == 400)
        #expect(fixture.requests.count == 1)
        await fixture.client.close()
    }

    @Test func sharesDecodedPixelsAcrossConcurrentAndRepeatedCards() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/art/movie")
        let loader = ArtworkLoader()
        async let first = loader.image(path: "/art/movie", client: fixture.client, dimension: 800)
        async let second = loader.image(path: "/art/movie", client: fixture.client, dimension: 800)
        let images = try await (first, second)
        let cached = try await loader.image(path: "/art/movie", client: fixture.client, dimension: 800)
        #expect(images.0 === images.1)
        #expect(images.0 === cached)
        #expect(fixture.requests.count == 1)
        #expect(max(cached.width, cached.height) <= 800)
        #expect(fixture.requests.first?.value(forHTTPHeaderField: "Authorization") == "Bearer fixture-token")
    }

    @Test func separatesSessionsAndRequestedSizesAndClearsCachedPixels() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/art/movie", width: 1600, height: 1000)
        let other = try await ServerClient(server: fixture.client.server, token: "another-token", protocolClasses: [FixtureURLProtocol.self])
        let loader = ArtworkLoader()
        let small = try await loader.image(path: "/art/movie", client: fixture.client, dimension: 800)
        let large = try await loader.image(path: "/art/movie", client: fixture.client, dimension: 1600)
        let isolated = try await loader.image(path: "/art/movie", client: other, dimension: 800)
        #expect(small.width == 800)
        #expect(large.width == 1600)
        #expect(isolated !== small)
        #expect(fixture.requests.count == 3)
        await loader.clear()
        let refreshed = try await loader.image(path: "/art/movie", client: fixture.client, dimension: 800)
        #expect(refreshed !== small)
        #expect(fixture.requests.count == 4)
        await other.close()
    }

    @Test func servesStaleDiskArtworkAndRefreshesItInTheBackground() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true), "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let fixture = try HTTPFixture(body: "{}", viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/art/movie", width: 800, height: 400)
        _ = try await ArtworkLoader().image(path: "/art/movie", client: fixture.client, dimension: 800)
        await fixture.client.close()
        try ageArtwork(in: directory)
        try installArtworkImage(fixture, path: "/art/movie", width: 600, height: 300)
        let reopened = try ServerClient(server: await fixture.client.server, viewer: viewer,
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        defer { Task { await reopened.close() } }
        let cached = try await ArtworkLoader().image(path: "/art/movie", client: reopened, dimension: 800)
        #expect(cached.width == 800)
        for _ in 0..<1_000 where fixture.requests.count < 2 { try await Task.sleep(for: .milliseconds(5)) }
        #expect(fixture.requests.count == 2)
    }

    @Test func staleRefreshesLeaveFetchSlotsForVisibleArtwork() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true),
                               "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let fixture = try HTTPFixture(body: "{}", viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/art/old", width: 800, height: 400)
        _ = try await ArtworkLoader().image(path: "/art/old", client: fixture.client, dimension: 800)
        await fixture.client.close()
        try ageArtwork(in: directory)
        let stalled = FixtureURLProtocol.Entry(data: Data(), status: 200, headers: [:], hold: true)
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.routes["/art/old"] = stalled }
        try installArtworkImage(fixture, path: "/art/visible")
        let reopened = try ServerClient(server: await fixture.client.server, viewer: viewer,
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        let loader = ArtworkLoader()
        for dimension in [400, 800, 1600, 4096] {
            _ = try await loader.image(path: "/art/old", client: reopened, dimension: dimension)
        }
        for _ in 0..<1_000 where fixture.requests.filter({ $0.url?.path == "/art/old" }).count < 2 {
            try await Task.sleep(for: .milliseconds(5))
        }
        #expect(fixture.requests.filter { $0.url?.path == "/art/old" }.count == 2) // original plus one shared refresh
        let visible = Task { try await loader.image(path: "/art/visible", client: reopened) }
        for _ in 0..<1_000 where !fixture.requests.contains(where: { $0.url?.path == "/art/visible" }) {
            try await Task.sleep(for: .milliseconds(5))
        }
        let started = fixture.requests.contains { $0.url?.path == "/art/visible" }
        #expect(started)
        if started { #expect(try await visible.value.width == 400) }
        else { visible.cancel() }
        await loader.clear()
        await reopened.close()
    }

    @Test func evictsLeastRecentlyUsedPixelsByDecodedMemoryCost() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let data = try artworkImageData(width: 1600, height: 1600)
        for index in 0..<4 { installArtwork(fixture, path: "/art/\(index)", data: data) }
        let loader = ArtworkLoader()
        // Four highly compressed images fit in the old encoded cache, but their
        // decoded pixels exceed 32 MiB. Touching the first protects it from eviction.
        for index in 0..<3 { _ = try await loader.image(path: "/art/\(index)", client: fixture.client) }
        _ = try await loader.image(path: "/art/0", client: fixture.client)
        _ = try await loader.image(path: "/art/3", client: fixture.client)
        _ = try await loader.image(path: "/art/0", client: fixture.client)
        _ = try await loader.image(path: "/art/1", client: fixture.client)
        #expect(fixture.requests.filter { $0.url?.path == "/art/0" }.count == 1)
        #expect(fixture.requests.filter { $0.url?.path == "/art/1" }.count == 2)
    }

    @Test func displaysLargePhotosWithoutRetainingThemOverTheCacheBudget() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/media/photo", width: 4096, height: 2200)
        let loader = ArtworkLoader()
        for _ in 0..<2 {
            let image = try await loader.image(path: "/media/photo", client: fixture.client, dimension: 4096)
            #expect(image.width == 4096)
            #expect(image.bytesPerRow * image.height > 32 * 1024 * 1024)
        }
        #expect(fixture.requests.count == 2)
    }

    @Test(arguments: ["", "/api/v1/me", "https://elsewhere.example/art/movie", "/art/../api/v1/me", "/episode-art/../api/v1/me"])
    func rejectsInvalidPathsBeforeNetwork(_ path: String) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await ArtworkLoader().image(path: path, client: fixture.client) }
        #expect(fixture.requests.isEmpty)
    }

    @Test(arguments: [-1, 0, 799, 1601, 4097, Int.max])
    func rejectsInvalidSizesBeforeNetwork(_ dimension: Int) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) {
            try await ArtworkLoader().image(path: "/art/movie", client: fixture.client, dimension: dimension)
        }
        #expect(fixture.requests.isEmpty)
    }

    @Test(arguments: [false, true])
    func retriesInvalidImagesWithoutCachingFailure(_ wrongType: Bool) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let valid = try artworkImageData()
        installArtwork(fixture, path: "/art/movie", data: wrongType ? valid : Data("broken image".utf8),
                type: wrongType ? "text/html" : "image/png")
        let loader = ArtworkLoader()
        await #expect(throws: ClientError.invalidResponse) { try await loader.image(path: "/art/movie", client: fixture.client) }
        installArtwork(fixture, path: "/art/movie", data: valid)
        _ = try await loader.image(path: "/art/movie", client: fixture.client)
        #expect(fixture.requests.count == 2)
    }

    @Test func clearingCancelsInFlightImagesAndAllowsANewRequest() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        FixtureURLProtocol.entries.withLock { $0[fixture.host]?.hold = true }
        let loader = ArtworkLoader()
        let first = Task { try await loader.image(path: "/art/movie", client: fixture.client) }
        defer { first.cancel() }
        for _ in 0..<200 where fixture.requests.isEmpty { try await Task.sleep(for: .milliseconds(5)) }
        #expect(fixture.requests.count == 1)
        await loader.clear()
        await #expect(throws: CancellationError.self) { try await first.value }
        try installArtworkImage(fixture, path: "/art/movie")
        _ = try await loader.image(path: "/art/movie", client: fixture.client)
        #expect(fixture.requests.count == 2)
    }

    @Test func cancelledCallDoesNotFetchOrReturnCachedArtwork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/art/movie")
        let loader = ArtworkLoader()
        _ = try await loader.image(path: "/art/movie", client: fixture.client)
        for path in ["/art/movie", "/art/new"] {
            let task = Task {
                withUnsafeCurrentTask { $0?.cancel() }
                return try await loader.image(path: path, client: fixture.client)
            }
            await #expect(throws: CancellationError.self) { try await task.value }
        }
        #expect(fixture.requests.count == 1)
    }

    @Test func preservesOrientationTransparencyAndEncodedReaderOutput() throws {
        let data = try artworkImageData(width: 400, height: 200, orientation: 6)
        let decoded = try ArtworkLoader.decodedThumbnail(data, dimension: 800)
        #expect(decoded.width == 200)
        #expect(decoded.height == 400)
        #expect(![CGImageAlphaInfo.none, .noneSkipFirst, .noneSkipLast].contains(decoded.alphaInfo))
        let encoded = try ArtworkLoader.thumbnail(data, dimension: 4096)
        let source = try #require(CGImageSourceCreateWithData(encoded as CFData, nil))
        #expect(CGImageSourceGetType(source) as String? == UTType.png.identifier)
        let restored = try #require(CGImageSourceCreateImageAtIndex(source, 0, nil))
        #expect(restored.width == 200)
        #expect(restored.height == 400)
    }

    @Test func rejectsEmptyOversizedAndMalformedImageData() {
        for data in [Data(), Data("invalid".utf8), Data(repeating: 0, count: 32 * 1024 * 1024 + 1)] {
            #expect(throws: ClientError.invalidResponse) { try ArtworkLoader.decodedThumbnail(data, dimension: 800) }
        }
    }

}
