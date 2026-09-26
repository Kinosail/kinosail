import Foundation
import ImageIO
import Testing
import UniformTypeIdentifiers
@testable import KinosailPlayer

struct LocalMediaCacheTests {
    private let scope = String(repeating: "a", count: 64)
    private let data = Data("{\"items\":[]}".utf8)

    @Test func queuedWritesFinishOnCloseAndAreDiscardedOnPurge() async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        await cache.enqueueWrite(data, key: "saved", kind: .catalog)
        await cache.close(purge: false)
        let reopened = try LocalMediaCache(scope: scope, directory: directory)
        #expect(await reopened.read("saved", kind: .catalog)?.data == data)
        await reopened.enqueueWrite(data, key: "purged", kind: .catalog)
        await reopened.close(purge: true)
        let afterPurge = try LocalMediaCache(scope: scope, directory: directory)
        #expect(await afterPurge.read("saved", kind: .catalog) == nil)
        #expect(await afterPurge.read("purged", kind: .catalog) == nil)
    }

    @Test func queuedInvalidWritesCreateNoFiles() async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        await cache.enqueueWrite(data, key: "bad\nkey", kind: .catalog)
        await cache.enqueueWrite(Data(), key: "empty", kind: .catalog)
        await cache.enqueueWrite(Data(repeating: 1, count: 2 * 1024 * 1024 + 1), key: "large", kind: .catalog)
        await cache.close(purge: false)
        #expect(!FileManager.default.fileExists(atPath: directory.path))
    }

    @Test func pageInvalidationPreservesSiblingWritesAndSurvivesRestart() async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        let revision = await cache.revision
        let pagesRevision = await cache.pagesRevision
        let first = "/api/v1/library?view=history&offset=0"
        let sibling = "/api/v1/library?sort=added&offset=0"
        let later = "/api/v1/library?offset=60"
        try await cache.write(data, key: first, kind: .catalog, revision: revision)
        try await cache.write(data, key: later, kind: .catalog, revision: revision)
        await cache.invalidatePages()
        try await cache.write(data, key: sibling, kind: .catalog, revision: revision)
        try await cache.write(Data("late".utf8), key: later, kind: .catalog,
                              revision: revision, pagesRevision: pagesRevision)
        #expect(await cache.read(first, kind: .catalog)?.fresh == true)
        #expect(await cache.read(sibling, kind: .catalog)?.fresh == true)
        #expect(await cache.read(later, kind: .catalog)?.data == data)
        #expect(await cache.read(later, kind: .catalog)?.fresh == false)
        await cache.close(purge: false)
        let restarted = try LocalMediaCache(scope: scope, directory: directory)
        #expect(await restarted.read(sibling, kind: .catalog)?.fresh == true)
        #expect(await restarted.read(later, kind: .catalog)?.fresh == false)
        await restarted.close(purge: false)
    }

    @Test func persistsAcrossInstancesAndInvalidationSurvivesRestart() async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        try await cache.write(data, key: "/api/v1/library", kind: .catalog)
        #expect(await cache.read("/api/v1/library", kind: .catalog)?.fresh == true)
        await cache.invalidateCatalog()
        await cache.close(purge: false)
        let restarted = try LocalMediaCache(scope: scope, directory: directory)
        let saved = try #require(await restarted.read("/api/v1/library", kind: .catalog))
        #expect(saved.data == data)
        #expect(!saved.fresh)
    }

    @Test func keepsSavedArtworkUsableWhileMarkingItStale() async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        try await cache.write(Data("image".utf8), key: "/art/movie", kind: .artwork)
        await cache.close(purge: false)
        let file = try #require(files(directory).first)
        var bytes = try Data(contentsOf: file)
        var timestamp = Date().addingTimeInterval(-(LocalMediaCache.artworkFreshLifetime + 1)).timeIntervalSince1970.bitPattern.bigEndian
        withUnsafeBytes(of: &timestamp) { bytes.replaceSubrange(40..<48, with: $0) }
        try bytes.write(to: file)
        let restarted = try LocalMediaCache(scope: scope, directory: directory)
        let saved = try #require(await restarted.read("/art/movie", kind: .artwork))
        #expect(saved.data == Data("image".utf8))
        #expect(!saved.fresh)
    }

    @Test func discardsLateWritesAfterInvalidationAndClose() async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        let ticket = await cache.revision
        await cache.invalidateCatalog()
        try await cache.write(data, key: "late", kind: .catalog, revision: ticket)
        #expect(await cache.read("late", kind: .catalog) == nil)
        await cache.close(purge: true)
        try await cache.write(data, key: "closed", kind: .catalog)
        let restarted = try LocalMediaCache(scope: scope, directory: directory)
        #expect(await restarted.read("closed", kind: .catalog) == nil)
    }

    @Test func rejectsInvalidInputBeforeCreatingFiles() async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        for scope in ["", "../outside", String(repeating: "g", count: 64), String(repeating: "a", count: 65)] {
            #expect(throws: ClientError.self) { try LocalMediaCache(scope: scope, directory: directory) }
        }
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        for key in ["", "invalid\nkey", String(repeating: "x", count: 16_385)] {
            await #expect(throws: ClientError.self) { try await cache.write(data, key: key, kind: .catalog) }
        }
        for bytes in [Data(), Data(repeating: 0, count: 2 * 1024 * 1024 + 1)] {
            await #expect(throws: ClientError.self) { try await cache.write(bytes, key: "oversized", kind: .catalog) }
        }
        #expect(!FileManager.default.fileExists(atPath: directory.path))
    }

    @Test(arguments: ["version", "truncated", "future", "expired", "oversized"])
    func treatsDamagedFilesAsCacheMisses(_ corruption: String) async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        try await cache.write(data, key: "catalog", kind: .catalog)
        await cache.close(purge: false)
        let file = try #require(files(directory).first)
        var bytes = try Data(contentsOf: file)
        switch corruption {
        case "version": bytes[7] = 255
        case "truncated": bytes = Data(bytes.prefix(47))
        case "oversized": bytes = Data(repeating: 0, count: 2 * 1024 * 1024 + 49)
        default:
            var timestamp = Date().addingTimeInterval(corruption == "future" ? 3600 : -31 * 86_400).timeIntervalSince1970.bitPattern.bigEndian
            withUnsafeBytes(of: &timestamp) { bytes.replaceSubrange(40..<48, with: $0) }
        }
        try bytes.write(to: file)
        let restarted = try LocalMediaCache(scope: scope, directory: directory)
        #expect(await restarted.read("catalog", kind: .catalog) == nil)
    }

    @Test func bindsFileContentsToBothResourceAndProfile() async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        try await cache.write(data, key: "one", kind: .catalog)
        let one = try #require(files(directory).first)
        try await cache.write(data, key: "two", kind: .catalog)
        let two = try #require(files(directory).first { $0 != one })
        try Data(contentsOf: one).write(to: two)
        await cache.close(purge: false)
        let restarted = try LocalMediaCache(scope: scope, directory: directory)
        #expect(await restarted.read("two", kind: .catalog) == nil)
        let otherScope = String(repeating: "b", count: 64)
        let other = try LocalMediaCache(scope: otherScope, directory: directory)
        try await other.write(data, key: "one", kind: .catalog)
        let otherFile = directory.appendingPathComponent(otherScope).appendingPathComponent(one.lastPathComponent)
        try Data(contentsOf: one).write(to: otherFile)
        await other.close(purge: false)
        #expect(await (try LocalMediaCache(scope: otherScope, directory: directory)).read("one", kind: .catalog) == nil)
    }

    @Test func refusesSymlinkedProfileRootsWithoutTouchingTheirTargets() async throws {
        let directory = temporary(), outside = temporary()
        defer { try? FileManager.default.removeItem(at: directory); try? FileManager.default.removeItem(at: outside) }
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: outside, withIntermediateDirectories: true)
        try FileManager.default.createSymbolicLink(at: directory.appendingPathComponent(scope), withDestinationURL: outside)
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        await #expect(throws: ClientError.self) { try await cache.write(data, key: "one", kind: .catalog) }
        #expect(await cache.read("one", kind: .catalog) == nil)
        await cache.close(purge: true)
        #expect(try FileManager.default.contentsOfDirectory(atPath: outside.path).isEmpty)
        #expect(FileManager.default.fileExists(atPath: outside.path))
    }

    @Test func boundsEntryCountAndBytesWithEviction() async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        let cache = try LocalMediaCache(scope: scope, directory: directory)
        for index in 0..<513 { try await cache.write(data, key: "small-\(index)", kind: .catalog) }
        #expect(try files(directory).count <= 512)
        let large = Data(repeating: 1, count: 2 * 1024 * 1024)
        for index in 0..<34 { try await cache.write(large, key: "large-\(index)", kind: .catalog) }
        let bytes = try files(directory).reduce(0) { total, file in total + (try file.resourceValues(forKeys: [.fileSizeKey]).fileSize ?? 0) }
        #expect(bytes <= 64 * 1024 * 1024)
        #expect(await cache.read("large-0", kind: .catalog) == nil)
        #expect(await cache.read("large-33", kind: .catalog)?.data.count == large.count)
    }

    @Test(arguments: ["/art/movie", "/art/movie?variant=episode", "/person/movie/0?scope=show"])
    func artworkSurvivesRestartAndServesDifferentThumbnailSizesWithoutNetwork(_ path: String) async throws {
        let directory = temporary()
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true), "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let fixture = try HTTPFixture(body: "{}", viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        let context = try #require(CGContext(data: nil, width: 1200, height: 800, bitsPerComponent: 8, bytesPerRow: 0,
                                            space: CGColorSpaceCreateDeviceRGB(), bitmapInfo: CGImageAlphaInfo.noneSkipLast.rawValue))
        let image = try #require(context.makeImage())
        let output = NSMutableData()
        let destination = try #require(CGImageDestinationCreateWithData(output, UTType.png.identifier as CFString, 1, nil))
        CGImageDestinationAddImage(destination, image, nil)
        #expect(CGImageDestinationFinalize(destination))
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes[URLComponents(string: path)!.path] = .init(data: output as Data, status: 200, headers: ["Content-Type": "image/png"])
        }
        let first = try await ArtworkLoader().image(path: path, client: fixture.client, dimension: 800)
        #expect(first.width == 800)
        await fixture.client.close()
        let server = await fixture.client.server
        let reopened = try ServerClient(server: server, viewer: viewer,
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        let second = try await ArtworkLoader().image(path: path, client: reopened, dimension: 1600)
        #expect(second.width == 1200)
        #expect(fixture.requests.count == 1)
        await reopened.close()
    }

    private func files(_ directory: URL) throws -> [URL] {
        guard let listing = FileManager.default.enumerator(at: directory, includingPropertiesForKeys: nil) else { return [] }
        return listing.compactMap { $0 as? URL }.filter { $0.pathExtension == "cache" }
    }
    private func temporary() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString) }
}
