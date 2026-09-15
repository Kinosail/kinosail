import Foundation
import Testing
@testable import KinosailPlayer

struct CollectionNameTests {
    @Test func metadataNamesRemainVisibleAndUseOneEncodedRouteSegment() async throws {
        let name = "28 Days/Weeks/Years Later Collection"
        let fixture = try HTTPFixture(body: "{\"collections\":[\"\(name)\",\"300 Collection\"]}")
        defer { fixture.remove() }
        #expect(try await fixture.client.collections() == [name, "300 Collection"])
        let detail = try HTTPFixture(body: "{\"name\":\"\(name)\",\"items\":[]}")
        defer { detail.remove() }
        #expect(try await detail.client.collection(name: name).isEmpty)
        #expect(detail.requests.last?.url?.absoluteString == "https://\(detail.host)/api/v1/collections/28%20Days%2FWeeks%2FYears%20Later%20Collection")
        #expect(detail.requests.allSatisfy { $0.httpMethod == "GET" })
        #expect(fixture.requests.allSatisfy { $0.httpMethod == "GET" })
    }

    @Test(arguments: ["", " ", ".", "..", "../private", "a/../b", "a/./b", "a\\b", " padded", "bad\nname", String(repeating: "a", count: 65)])
    func rejectsInvalidNamesBeforeNetwork(_ name: String) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.collection(name: name) }
        #expect(fixture.requests.isEmpty)
    }

    @Test(arguments: ["/api/v1/collections/a/b", "/api/v1/collections/a%2fb", "/api/v1/collections/a?extra=1", "/api/v1/collections/%2E%2E%2Fprivate"])
    func rejectsNoncanonicalCollectionRoutesBeforeNetwork(_ path: String) async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.request(path) }
        #expect(fixture.requests.isEmpty)
    }

    @Test func encodedSlashesRemainRejectedOutsideCollectionReads() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.request("/api/v1/items/a%2Fb") }
        await #expect(throws: ClientError.self) { try await fixture.client.request("/api/v1/collections/a%2Fb", method: .delete) }
        #expect(fixture.requests.isEmpty)
    }

    @Test(arguments: ["{}", "{\"collections\":[null]}", "{\"collections\":[\"same\",\"same\"]}", "{\"collections\":[],\"unknown\":true}"])
    func rejectsMalformedCollectionResponsesWithoutWrites(_ body: String) async throws {
        let fixture = try HTTPFixture(body: body)
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.collections() }
        #expect(fixture.requests.count == 1)
        #expect(fixture.requests.allSatisfy { $0.httpMethod == "GET" })
    }
}
