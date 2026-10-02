import Foundation
import ImageIO
import Testing
@testable import KinosailPlayer

struct ArtworkAdmissionTests {
    @Test func foregroundDemandDoesNotPromoteAnotherDecodedSize() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true),
                               "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let fixture = try HTTPFixture(body: "{}", viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        let large = try artworkImageData(width: 1600, height: 1600)
        for index in 0..<3 {
            installArtwork(fixture, path: "/art/visible\(index)", data: large)
            _ = try await ArtworkLoader().image(path: "/art/visible\(index)", client: fixture.client)
        }
        await fixture.client.close()
        let server = await fixture.client.server
        ControlledArtworkProtocol.entries.withLock { $0[fixture.host] = .init(data: large) }
        defer { ControlledArtworkProtocol.entries.withLock { $0[fixture.host] = nil } }
        let client = try ServerClient(server: server, viewer: viewer, protocolClasses: [ControlledArtworkProtocol.self], cacheDirectory: directory)
        let loader = ArtworkLoader()
        var visible: [CGImage] = []
        for index in 0..<3 { visible.append(try await loader.image(path: "/art/visible\(index)", client: client)) }
        let foreground = Task { try await loader.image(path: "/art/shared-size", client: client, dimension: 800) }
        for _ in 0..<200 where ControlledArtworkProtocol.active(fixture.host).isEmpty { try await Task.sleep(for: .milliseconds(5)) }
        let pending = try #require(ControlledArtworkProtocol.active(fixture.host).first)
        let store = try #require(await client.cacheStore())
        try await store.write(large, key: server.mediaURL("/art/shared-size").absoluteString, kind: .artwork)
        _ = try await loader.image(path: "/art/shared-size", client: client, dimension: 1600, background: true)
        for index in 0..<3 {
            let again = try await loader.image(path: "/art/visible\(index)", client: client)
            #expect(again === visible[index])
        }
        pending.finish()
        #expect(try await foreground.value.width == 800)
        await loader.clear()
        await client.close()
    }

    @Test(arguments: [1600, 4096])
    func growingBackgroundRefreshKeepsOtherVisiblePixels(_ dimension: Int) async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true),
                               "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
        let fixture = try HTTPFixture(body: "{}", viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        try installArtworkImage(fixture, path: "/art/old", width: 800, height: 800)
        _ = try await ArtworkLoader().image(path: "/art/old", client: fixture.client)
        await fixture.client.close()
        try ageArtwork(in: directory)
        let server = await fixture.client.server
        let seed = try ServerClient(server: server, viewer: viewer, protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        for index in 0..<3 {
            try installArtworkImage(fixture, path: "/art/visible\(index)", width: 1600, height: 1600)
            _ = try await ArtworkLoader().image(path: "/art/visible\(index)", client: seed)
        }
        await seed.close()
        let large = try artworkImageData(width: dimension, height: dimension)
        ControlledArtworkProtocol.entries.withLock { $0[fixture.host] = .init(data: large) }
        defer { ControlledArtworkProtocol.entries.withLock { $0[fixture.host] = nil } }
        let client = try ServerClient(server: server, viewer: viewer, protocolClasses: [ControlledArtworkProtocol.self], cacheDirectory: directory)
        let loader = ArtworkLoader()
        var visible: [CGImage] = []
        for index in 0..<3 { visible.append(try await loader.image(path: "/art/visible\(index)", client: client)) }
        let old = try await loader.image(path: "/art/old", client: client, dimension: dimension)
        #expect(old.width == 800)
        for _ in 0..<200 where ControlledArtworkProtocol.active(fixture.host).isEmpty { try await Task.sleep(for: .milliseconds(5)) }
        let pending = try #require(ControlledArtworkProtocol.active(fixture.host).first)
        pending.finish()
        let store = try #require(await client.cacheStore())
        let key = try server.mediaURL("/art/old").absoluteString
        var saved: LocalMediaCache.Entry?
        for _ in 0..<200 {
            saved = await store.read(key, kind: .artwork)
            if saved?.fresh == true { break }
            try await Task.sleep(for: .milliseconds(5))
        }
        #expect(saved?.data == large)
        for index in 0..<3 {
            let again = try await loader.image(path: "/art/visible\(index)", client: client)
            #expect(again === visible[index])
        }
        let refreshed = try await loader.image(path: "/art/old", client: client, dimension: dimension)
        #expect(refreshed.width == dimension)
        await loader.clear()
        await client.close()
    }

    @Test func backgroundPixelsDoNotEvictVisiblePixels() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        let data = try artworkImageData(width: 1600, height: 1600)
        for index in 0..<6 { installArtwork(fixture, path: "/art/admission\(index)", data: data) }
        let loader = ArtworkLoader()
        let visible = try await loader.image(path: "/art/admission0", client: fixture.client)
        for index in 1..<6 {
            _ = try await loader.image(path: "/art/admission\(index)", client: fixture.client, background: true)
        }
        let again = try await loader.image(path: "/art/admission0", client: fixture.client)
        #expect(again === visible)
        #expect(fixture.requests.filter { $0.url?.path == "/art/admission0" }.count == 1)
        // A background hit becomes foreground-owned when it is displayed.
        let promoted = try await loader.image(path: "/art/admission5", client: fixture.client)
        for index in 1..<5 {
            _ = try await loader.image(path: "/art/admission\(index)", client: fixture.client, background: true)
        }
        let promotedAgain = try await loader.image(path: "/art/admission5", client: fixture.client)
        #expect(promotedAgain === promoted)
        await fixture.client.close()
    }
}
