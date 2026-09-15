import Foundation
import Testing
@testable import KinosailPlayer

struct CatalogCacheTests {
    @Test func persistsRealEpisodeAndShowArtworkVariants() async throws {
        let directory = cacheDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let id = String(repeating: "a", count: 16)
        let body = """
        {"id":"\(id)","title":"Series","backdrop":"/backdrop/movie","play":"/watch/movie","cast":[{"name":"Actor","image":"/person/movie/0?scope=show"}],"episodes":[{"id":"movie","kind":"video","title":"Episode","show":"Series","showId":"\(id)","season":1,"episode":1,"artwork":"/art/movie?variant=episode","stream":"/media/movie","progress":{}}]}
        """
        let fixture = try HTTPFixture(body: body, viewer: profile(), cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.episodes(showID: id, policy: .automatic)
        await fixture.client.close()
        let reopened = try ServerClient(server: fixture.client.server, viewer: profile(),
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        let episodes = try await reopened.episodes(showID: id, policy: .automatic)
        #expect(episodes.first?.artwork == "/art/movie?variant=episode")
        #expect(fixture.requests.count == 1)
        await reopened.close()
    }

    @Test func survivesClientRestartWithoutRepeatingTheLibraryRequest() async throws {
        let directory = cacheDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try profile()
        let fixture = try HTTPFixture(body: library(), viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.library(policy: .automatic)
        await fixture.client.close()
        let reopened = try ServerClient(server: fixture.client.server, token: "new-token", viewer: viewer,
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        let page = try await reopened.library(policy: .automatic)
        #expect(page.items.first?.id == "movie")
        #expect(fixture.requests.count == 1)
        let cached = try await reopened.library(policy: .cached)
        #expect(cached.items.first?.id == "movie")
        #expect(fixture.requests.count == 1)
        await reopened.close()
    }

    @Test func separatesProfileFiltersQueriesAndPageOffsets() async throws {
        let directory = cacheDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: library(), viewer: profile(), cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.library(policy: .automatic)
        for operation in [
            { try await fixture.client.library(query: "other", policy: .cached) },
            { try await fixture.client.library(view: .shows, policy: .cached) },
            { try await fixture.client.library(sort: .added, policy: .cached) },
            { try await fixture.client.library(offset: 60, policy: .cached) }
        ] {
            await #expect(throws: CatalogCacheMiss.self) { try await operation() }
        }
        let other = try ServerClient(server: fixture.client.server, token: "other-token", viewer: profile(id: "other"),
                                     protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        await #expect(throws: CatalogCacheMiss.self) { try await other.library(policy: .cached) }
        #expect(fixture.requests.count == 1)
        await other.close()
        await fixture.client.close()
    }

    @Test func servesSavedContentThenRefreshesAfterInvalidationAndManualReload() async throws {
        let directory = cacheDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: library(), viewer: profile(), cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.library(policy: .automatic)
        await fixture.client.invalidateCatalog()
        let cached = try await fixture.client.library(policy: .cached)
        #expect(cached.items.first?.title == "Movie")
        #expect(fixture.requests.count == 1)
        setLibrary(fixture, body: library(title: "Updated"))
        let fresh = try await fixture.client.library(policy: .automatic)
        #expect(fresh.items.first?.title == "Updated")
        #expect(fixture.requests.count == 2)
        _ = try await fixture.client.library(policy: .automatic)
        #expect(fixture.requests.count == 2)
        _ = try await fixture.client.library(policy: .reload)
        #expect(fixture.requests.count == 3)
        await fixture.client.close()
    }

    @Test func changedFirstPageRequiresRefreshingLaterOffsets() async throws {
        let directory = cacheDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: library(), viewer: profile(), cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.library(policy: .automatic)
        setLibrary(fixture, body: library(offset: 60))
        _ = try await fixture.client.library(offset: 60, policy: .automatic)
        setLibrary(fixture, body: library(title: "Inserted"))
        _ = try await fixture.client.library(policy: .reload)
        setLibrary(fixture, body: library(title: "Shifted", offset: 60))
        let page = try await fixture.client.library(offset: 60, policy: .automatic)
        #expect(page.items.first?.title == "Shifted")
        #expect(fixture.requests.count == 4)
        await fixture.client.close()
    }

    @Test func mutationInvalidatesCatalogButPreservesImmediateCachedDisplay() async throws {
        let directory = cacheDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: library(), viewer: profile(), cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.library(policy: .automatic)
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/api/v1/items/movie/list"] = .init(data: Data("{\"listed\":true}".utf8), status: 200, headers: [:])
        }
        _ = try await fixture.client.setListed(itemID: "movie", listed: true)
        _ = try await fixture.client.library(policy: .cached)
        #expect(fixture.requests.count == 2)
        _ = try await fixture.client.library(policy: .automatic)
        #expect(fixture.requests.count == 3)
        await fixture.client.close()
    }

    @Test func coalescesConcurrentLoadsAndDoesNotPersistInvalidResponses() async throws {
        let directory = cacheDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: "{}", viewer: profile(), cacheDirectory: directory)
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.library(policy: .automatic) }
        await #expect(throws: CatalogCacheMiss.self) { try await fixture.client.library(policy: .cached) }
        setLibrary(fixture, body: library())
        async let first = fixture.client.library(policy: .automatic)
        async let second = fixture.client.library(policy: .automatic)
        let pages = try await (first, second)
        #expect(pages.0.items.first?.id == pages.1.items.first?.id)
        #expect(fixture.requests.count == 2)
        await fixture.client.close()
    }

    @Test(arguments: [401, 403, 404])
    func removesRevokedOrDeletedContentInsteadOfFallingBack(_ status: Int) async throws {
        let directory = cacheDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: library(), viewer: profile(), cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.library(policy: .automatic)
        setLibrary(fixture, body: "{}", status: status)
        await #expect(throws: ClientError.http(status)) { try await fixture.client.library(policy: .reload) }
        await #expect(throws: CatalogCacheMiss.self) { try await fixture.client.library(policy: .cached) }
        await fixture.client.close()
    }

    @Test func signOutPurgesEvenWhenTheServerIsUnavailable() async throws {
        let directory = cacheDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try profile()
        let fixture = try HTTPFixture(body: library(), viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.library(policy: .automatic)
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/api/v1/session"] = .init(data: Data("{}".utf8), status: 503, headers: [:])
        }
        await #expect(throws: ClientError.http(503)) { try await fixture.client.signOut() }
        let reopened = try ServerClient(server: fixture.client.server, viewer: viewer,
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        await #expect(throws: CatalogCacheMiss.self) { try await reopened.library(policy: .cached) }
        await reopened.close(); await fixture.client.close()
    }

    @Test func refusesCapabilityPersistenceAndNonCatalogEndpoints() async throws {
        let directory = cacheDirectory()
        defer { try? FileManager.default.removeItem(at: directory) }
        let fixture = try HTTPFixture(body: library(stream: "/media/movie?token=private"), viewer: profile(), cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.library(policy: .automatic)
        await #expect(throws: CatalogCacheMiss.self) { try await fixture.client.library(policy: .cached) }
        for path in ["/api/v1/me", "/api/v1/items/movie/playback", "/api/v1/session", "/api/v1/downloads/job"] {
            await #expect(throws: ClientError.self) {
                try await fixture.client.catalog(path, policy: .automatic) { $0 }
            }
        }
        #expect(fixture.requests.count == 1)
        #expect(!FileManager.default.fileExists(atPath: directory.path))
        await fixture.client.close()
    }

    private func setLibrary(_ fixture: HTTPFixture, body: String, status: Int = 200) {
        FixtureURLProtocol.entries.withLock {
            $0[fixture.host]?.routes["/api/v1/library"] = .init(data: Data(body.utf8), status: status, headers: [:])
        }
    }
    private func library(title: String = "Movie", offset: Int = 0, stream: String = "/media/movie") -> String {
        """
        {"items":[{"id":"movie","kind":"video","title":"\(title)","stream":"\(stream)","progress":{}}],"view":"all","sort":"title","query":"","total":100,"offset":\(offset),"limit":60,"letters":[]}
        """
    }
    private func cacheDirectory() -> URL { FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString) }
    private func profile(id: String = "viewer") throws -> Viewer {
        try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string(id), "name": .string("Viewer"), "owner": .bool(true), "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
    }
}
