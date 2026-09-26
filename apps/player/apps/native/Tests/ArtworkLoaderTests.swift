import Foundation
import ImageIO
import Testing
import UniformTypeIdentifiers
@testable import KinosailPlayer

struct ArtworkLoaderTests {
    @Test func diskHitsRetainDecodedPixelsIncludingTopShelfSize() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true), "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let fixture = try HTTPFixture(body: "{}", viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        try installImage(fixture, path: "/art/movie", width: 800, height: 400)
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
        try installImage(fixture, path: "/art/movie")
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
        try installImage(fixture, path: "/art/movie", width: 1600, height: 1000)
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
        try installImage(fixture, path: "/art/movie", width: 800, height: 400)
        _ = try await ArtworkLoader().image(path: "/art/movie", client: fixture.client, dimension: 800)
        await fixture.client.close()
        try ageArtwork(in: directory)
        try installImage(fixture, path: "/art/movie", width: 600, height: 300)
        let reopened = try ServerClient(server: await fixture.client.server, viewer: viewer,
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        defer { Task { await reopened.close() } }
        let cached = try await ArtworkLoader().image(path: "/art/movie", client: reopened, dimension: 800)
        #expect(cached.width == 800)
        for _ in 0..<1_000 where fixture.requests.count < 2 { try await Task.sleep(for: .milliseconds(5)) }
        #expect(fixture.requests.count == 2)
    }

    @Test func evictsLeastRecentlyUsedPixelsByDecodedMemoryCost() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let data = try imageData(width: 1600, height: 1600)
        for index in 0..<4 { install(fixture, path: "/art/\(index)", data: data) }
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
        try installImage(fixture, path: "/media/photo", width: 4096, height: 2200)
        let loader = ArtworkLoader()
        for _ in 0..<2 {
            let image = try await loader.image(path: "/media/photo", client: fixture.client, dimension: 4096)
            #expect(image.width == 4096)
            #expect(image.bytesPerRow * image.height > 32 * 1024 * 1024)
        }
        #expect(fixture.requests.count == 2)
    }

    @Test(arguments: ["", "/api/v1/me", "https://elsewhere.example/art/movie", "/art/../api/v1/me"])
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
        let valid = try imageData()
        install(fixture, path: "/art/movie", data: wrongType ? valid : Data("broken image".utf8),
                type: wrongType ? "text/html" : "image/png")
        let loader = ArtworkLoader()
        await #expect(throws: ClientError.invalidResponse) { try await loader.image(path: "/art/movie", client: fixture.client) }
        install(fixture, path: "/art/movie", data: valid)
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
        try installImage(fixture, path: "/art/movie")
        _ = try await loader.image(path: "/art/movie", client: fixture.client)
        #expect(fixture.requests.count == 2)
    }

    @Test func cancelledCallDoesNotFetchOrReturnCachedArtwork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        try installImage(fixture, path: "/art/movie")
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
        let data = try imageData(width: 400, height: 200, orientation: 6)
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

    private func installImage(_ fixture: HTTPFixture, path: String, width: Int = 400, height: Int = 200) throws {
        install(fixture, path: path, data: try imageData(width: width, height: height))
    }

    private func install(_ fixture: HTTPFixture, path: String, data: Data, type: String = "image/png") {
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes[path] = .init(data: data, status: 200, headers: ["Content-Type": type])
        }
    }

    private func imageData(width: Int = 400, height: Int = 200, orientation: Int = 1) throws -> Data {
        let context = try #require(CGContext(data: nil, width: width, height: height, bitsPerComponent: 8, bytesPerRow: 0,
                                            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue))
        context.setFillColor(CGColor(red: 0.2, green: 0.8, blue: 0.4, alpha: 0.5))
        context.fill(CGRect(x: 0, y: 0, width: width, height: height))
        let image = try #require(context.makeImage())
        let output = NSMutableData()
        let destination = try #require(CGImageDestinationCreateWithData(output, UTType.png.identifier as CFString, 1, nil))
        CGImageDestinationAddImage(destination, image, [kCGImagePropertyOrientation: orientation] as CFDictionary)
        #expect(CGImageDestinationFinalize(destination))
        return output as Data
    }

    private func ageArtwork(in directory: URL) throws {
        guard let file = FileManager.default.enumerator(at: directory, includingPropertiesForKeys: nil)?
            .compactMap({ $0 as? URL }).first(where: { $0.pathExtension == "cache" }) else { throw ClientError.invalidResponse }
        var bytes = try Data(contentsOf: file)
        var timestamp = Date().addingTimeInterval(-(LocalMediaCache.artworkFreshLifetime + 1)).timeIntervalSince1970.bitPattern.bigEndian
        withUnsafeBytes(of: &timestamp) { bytes.replaceSubrange(40..<48, with: $0) }
        try bytes.write(to: file)
    }
}
