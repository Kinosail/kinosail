import Foundation
import Testing
@testable import KinosailPlayer

@MainActor
struct AppLaunchCacheTests {
    @Test func reopensCustomMusicTabFromDiskWithoutARequest() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try profile()
        let fixture = try HTTPFixture(body: "{\"albums\":[{\"id\":\"album\",\"title\":\"Saved album\",\"artist\":\"Artist\"}]}",
                                      viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.albums(policy: .automatic)
        let server = await fixture.client.server
        await fixture.client.close()
        let listing = try #require(FileManager.default.enumerator(at: directory, includingPropertiesForKeys: nil))
        let file = try #require(listing.compactMap { $0 as? URL }.first { $0.pathExtension == "cache" })
        var data = try Data(contentsOf: file)
        var old = Date().addingTimeInterval(-2 * 86_400).timeIntervalSince1970.bitPattern.bigEndian
        withUnsafeBytes(of: &old) { data.replaceSubrange(40..<48, with: $0) }
        try data.write(to: file)
        let reopened = try ServerClient(server: server, viewer: viewer,
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        let clientID = await reopened.identity
        let defaults = try #require(UserDefaults(suiteName: "launch-cache-\(UUID().uuidString)"))
        let profileKey = "music-profile"
        defaults.set(PlayerMode.listen.rawValue, forKey: PlayerMode.storageKey(profileKey))
        defaults.set("music,home", forKey: PlayerMode.listen.tabsKey(profileKey))
        let snapshots = ResourceSnapshotCache()
        await AppLaunchCache.hydrate(client: reopened, profileKey: profileKey, snapshots: snapshots, defaults: defaults)
        #expect(snapshots.value(for: profileKey, clientID: clientID, as: [Album].self)?.first?.title == "Saved album")
        #expect(!snapshots.isFresh(for: profileKey, clientID: clientID, as: [Album].self, refreshID: "current"))
        #expect(fixture.requests.count == 1)
        let abandoned = ResourceSnapshotCache()
        var checks = 0
        await AppLaunchCache.hydrate(client: reopened, profileKey: profileKey, snapshots: abandoned, defaults: defaults,
                                     ifCurrent: { checks += 1; return checks == 1 })
        #expect(abandoned.value(for: profileKey, clientID: clientID, as: [Album].self) == nil)
        #expect(fixture.requests.count == 1)
        _ = try await reopened.albums(policy: .automatic)
        #expect(fixture.requests.count == 2)
        await reopened.close()
    }

    @Test func reopensCustomLibraryTabAndRejectsInvalidTabPreferenceWithoutNetwork() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try profile()
        let body = "{\"items\":[{\"id\":\"movie\",\"kind\":\"video\",\"title\":\"Saved movie\"}],\"total\":1,\"offset\":0,\"limit\":60}"
        let fixture = try HTTPFixture(body: body, viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        _ = try await fixture.client.library(view: .movies, policy: .automatic)
        let server = await fixture.client.server
        await fixture.client.close()
        let reopened = try ServerClient(server: server, viewer: viewer,
                                        protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        let clientID = await reopened.identity
        let defaults = try #require(UserDefaults(suiteName: "launch-cache-\(UUID().uuidString)"))
        let profileKey = "movies-profile"
        defaults.set("movies,home", forKey: PlayerMode.watch.tabsKey(profileKey))
        let snapshots = ResourceSnapshotCache()
        await AppLaunchCache.hydrate(client: reopened, profileKey: profileKey, snapshots: snapshots, defaults: defaults)
        #expect(snapshots.value(for: LibrarySnapshot.key(view: .movies), clientID: clientID,
                                as: LibrarySnapshot.self)?.items.first?.title == "Saved movie")
        #expect(fixture.requests.count == 1)

        defaults.set(String(repeating: "x", count: 129), forKey: PlayerMode.watch.tabsKey(profileKey))
        let rejected = ResourceSnapshotCache()
        await AppLaunchCache.hydrate(client: reopened, profileKey: profileKey, snapshots: rejected, defaults: defaults)
        #expect(rejected.value(for: LibrarySnapshot.key(view: .movies), clientID: clientID,
                               as: LibrarySnapshot.self) == nil)
        #expect(fixture.requests.count == 1)
        await reopened.close()
    }

    private func profile() throws -> Viewer {
        try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string("viewer"), "name": .string("Viewer"), "owner": .bool(true),
                               "downloads": .bool(true), "transcode": .bool(true), "remote": .bool(false)])]))
    }
}
