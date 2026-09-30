import Foundation
import Testing
@testable import KinosailPlayer

struct WatchProgressSummaryTests {
    @Test func reusesProgressOnRepeatVisitsAndAfterRestart() async throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        defer { try? FileManager.default.removeItem(at: directory) }
        let viewer = try Self.viewer("viewer")
        let fixture = try HTTPFixture(body: "{\"seconds\":120,\"duration\":600}", viewer: viewer, cacheDirectory: directory)
        defer { fixture.remove() }
        #expect(try await fixture.client.watchProgress(itemID: "movie").fraction == 0.2)
        #expect(try await fixture.client.watchProgress(itemID: "movie").fraction == 0.2)
        #expect(fixture.requests.count == 1)
        await fixture.client.close()
        let reopened = try await ServerClient(server: fixture.client.server, viewer: viewer,
                                              protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        #expect(try await reopened.watchProgress(itemID: "movie").fraction == 0.2)
        #expect(fixture.requests.count == 1)
        await reopened.close()
        let other = try await ServerClient(server: fixture.client.server, viewer: Self.viewer("other"),
                                           protocolClasses: [FixtureURLProtocol.self], cacheDirectory: directory)
        _ = try await other.watchProgress(itemID: "movie")
        #expect(fixture.requests.count == 2)
        #expect(fixture.requests.last?.value(forHTTPHeaderField: "X-Kinosail-Viewer-Profile") == "other")
        await other.close()
    }

    @Test func invalidRemoteProgressDoesNotPoisonTheCache() async throws {
        let fixture = try HTTPFixture(body: "{\"seconds\":601,\"duration\":600}", viewer: Self.viewer("viewer"))
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.watchProgress(itemID: "movie") }
        FixtureURLProtocol.entries.withLock { values in
            values[fixture.host] = .init(data: Data("{\"seconds\":120,\"duration\":600}".utf8), status: 200, headers: [:],
                                         requests: values[fixture.host]?.requests ?? [])
        }
        #expect(try await fixture.client.watchProgress(itemID: "movie").fraction == 0.2)
        #expect(fixture.requests.count == 2)
        await fixture.client.close()
    }

    private static func viewer(_ id: String) throws -> Viewer {
        try Viewer(.object(["server": .string("Test"), "serverId": .string("test-server"),
            "viewer": .object(["id": .string(id), "name": .string("Viewer"), "owner": .bool(false),
                               "downloads": .bool(false), "transcode": .bool(false), "remote": .bool(false)])]))
    }

    @Test(arguments: ["", "../movie", String(repeating: "x", count: 129)])
    func rejectsInvalidItemWithoutNetwork(_ id: String) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.watchProgress(itemID: id) }
        #expect(fixture.requests.isEmpty)
    }

    @Test func reportsOnlyKnownDuration() throws {
        let known = try WatchProgressSummary(.object(["seconds": .number(120), "duration": .number(600)]))
        #expect(known.fraction == 0.2)
        #expect(known.remainingLabel == "8 min left")
        let unknown = try WatchProgressSummary(.object(["seconds": .number(120), "duration": .number(0)]))
        #expect(unknown.fraction == nil)
        #expect(unknown.remainingLabel == nil)
    }
    @Test(arguments: [
        JSONValue.object([:]), .object(["seconds": .number(1)]), .object(["duration": .number(1)]),
        .object(["seconds": .string("1"), "duration": .number(10)]),
        .object(["seconds": .number(-1), "duration": .number(10)]),
        .object(["seconds": .number(11), "duration": .number(10)]),
        .object(["seconds": .number(0), "duration": .number(315_360_001)]),
        .object(["seconds": .number(0), "duration": .number(.infinity)]),
        .object(["seconds": .number(.nan), "duration": .number(10)]),
        .object(["seconds": .number(0), "duration": .number(10), "unknown": .bool(true)])
    ])
    func rejectsInvalidSummary(_ raw: JSONValue) {
        #expect(throws: ClientError.self) { try WatchProgressSummary(raw) }
    }
}
