import Testing
@testable import KinosailPlayer

struct SupporterCollectionTests {
    private func badge(_ edition: String = "monthly", rank: Double = 10) -> JSONValue {
        .object(["edition": .string(edition), "family": .string(edition == "one-time" ? "patron-order" : "living-standard"),
                 "tier": .string("legacy"), "name": .string("Legacy"), "rank": .number(rank), "active": .bool(false), "archived": .bool(true)])
    }
    @Test func hiddenKeepsAllThreeEarnedBadges() throws {
        let collection = try SupporterCollection(.object(["badges": .array([badge("one-time"), badge(), badge("yearly")]), "display": .string("hidden")]))
        #expect(!collection.visible)
        #expect(collection.badges.count == 3)
        #expect(collection.badges.allSatisfy { $0.archived })
        #expect(collection.badges[1].artwork == "supporter-monthly-10")
    }
    @Test func rejectsMalformedRemoteState() {
        for input: JSONValue in [.object([:]), .null, .object(["display": .string("hidden"), "badges": .null]),
                                 .object(["display": .string("unknown"), "badges": .array([])]),
                                 .object(["display": .string("automatic"), "badges": .array([badge(), badge()])]),
                                 .object(["display": .string("automatic"), "badges": .array(Array(repeating: badge(), count: 5))])] {
            #expect(throws: ClientError.self) { try SupporterCollection(input) }
        }
        for rank in [0.0, -1, 1.5, 11] { #expect(throws: ClientError.self) { try SupporterBadge(badge(rank: rank)) } }
        for edition in ["MONTHLY", "unknown", " monthly", String(repeating: "a", count: 17)] {
            #expect(throws: ClientError.self) { try SupporterBadge(badge(edition)) }
        }
    }
}
