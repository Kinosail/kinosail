import Foundation
import Testing
@testable import KinosailPlayer

// Isolated pixel/cache checks protect against a false sharpness claim that a
// remote-focus screenshot cannot measure. They are not populated-server proof.
struct ArtworkBackdropResolutionTests {
    @Test func preserves4KBackdropPixelsIndependentlyOfThumbnailDecode() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/backdrop/movie", width: 3840, height: 2160)
        let loader = ArtworkLoader()
        let thumbnail = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 800)
        let hero = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        let repeated = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        #expect(thumbnail.width == 800)
        #expect(hero.width == 3840 && hero.height == 2160)
        #expect(hero !== thumbnail && hero === repeated)
        #expect(hero.bytesPerRow * hero.height <= 32 * 1024 * 1024)
        await loader.clear()
        await fixture.client.close()
    }

    @Test func highResolutionBackdropRefreshesLegacyDiskBytesOnceWithoutPurgingThumbnails() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true),
                               "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let fixture = try HTTPFixture(body: "{}", viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/backdrop/movie", width: 780, height: 439)
        let legacy = try await ArtworkLoader().image(path: "/backdrop/movie", client: fixture.client, dimension: 800)
        #expect(legacy.width == 780)
        if let store = try await fixture.client.cacheStore() { await store.flushWrites() }

        // The Server has upgraded this derivative; the legacy encoded download
        // is deliberately still fresh. No global cache clear precedes the hero.
        try installArtworkImage(fixture, path: "/backdrop/movie", width: 3840, height: 2160)
        let loader = ArtworkLoader()
        let hero = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        let repeated = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        #expect(hero.width == 3840 && hero.height == 2160)
        #expect(hero === repeated)
        #expect(fixture.requests.count == 2)
        if let store = try await fixture.client.cacheStore() { await store.flushWrites() }

        let nextLoader = ArtworkLoader()
        let diskHero = try await nextLoader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        let untouchedThumbnail = try await nextLoader.image(path: "/backdrop/movie", client: fixture.client, dimension: 800)
        #expect(diskHero.width == 3840 && diskHero.height == 2160)
        #expect(untouchedThumbnail.width == 780)
        #expect(fixture.requests.count == 2)
        await loader.clear()
        await nextLoader.clear()
        await fixture.client.close()
    }

    @Test func highResolutionRequestDoesNotUpscaleOrRepeatedlyFetchSmallSource() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/backdrop/movie", width: 780, height: 439)
        let loader = ArtworkLoader()
        let first = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        let repeated = try await loader.image(path: "/backdrop/movie", client: fixture.client, dimension: 4096)
        #expect(first.width == 780 && first.height == 439)
        #expect(first === repeated)
        #expect(fixture.requests.count == 1)
        await loader.clear()
        await fixture.client.close()
    }
}
