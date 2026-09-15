import Testing
@testable import KinosailPlayer

struct WatchProgressSummaryTests {
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
