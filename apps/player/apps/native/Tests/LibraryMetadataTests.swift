import Foundation
import Testing
@testable import KinosailPlayer

struct LibraryMetadataTests {
    @Test func importedSynopsisRemovesOnlyZeroWidthSpaces() throws {
        let server = try ServerAddress("https://media.example")
        let item = try MediaItem(.object([
            "id": .string("sample"), "kind": .string("video"), "title": .string("Sample"),
            "plot": .string("A quiet\u{200B} evening.\nCafé by the sea.")
        ]), server: server)
        #expect(item.plot == "A quiet evening.\nCafé by the sea.")
    }

    @Test func recentlyAddedPageAcceptsAnImportedSynopsis() async throws {
        let fixture = try HTTPFixture(body: page(plot: .string("An imported\u{200B} synopsis.")))
        defer { fixture.remove() }
        let result = try await fixture.client.library(view: .movies, sort: .added)
        #expect(result.items.map(\.plot) == ["An imported synopsis."])
        #expect(fixture.requests.count == 1)
        #expect(fixture.requests.allSatisfy { $0.httpMethod == "GET" })
    }

    @Test func malformedSynopsesRemainRejectedWithoutWrites() async throws {
        let invalid: [JSONValue] = [
            .null, .bool(true), .array([]), .string("bad\u{0000}text"),
            .string("bad\ttext"), .string("bad\u{202E}text"),
            .string(String(repeating: "a", count: 10_001)),
            .string(String(repeating: "\u{200B}", count: 3_334))
        ]
        for plot in invalid {
            let fixture = try HTTPFixture(body: page(plot: plot))
            defer { fixture.remove() }
            await #expect(throws: ClientError.self) { try await fixture.client.library(view: .movies, sort: .added) }
            #expect(fixture.requests.count == 1)
            #expect(fixture.requests.allSatisfy { $0.httpMethod == "GET" })
        }
    }

    @Test func zeroWidthSpacesRemainInvalidInIdentifiersAndTitles() throws {
        let server = try ServerAddress("https://media.example")
        for key in ["id", "title", "stream"] {
            var item: [String: JSONValue] = ["id": .string("sample"), "title": .string("Sample"), "kind": .string("video")]
            item[key] = .string("sample\u{200B}")
            #expect(throws: ClientError.self) { try MediaItem(.object(item), server: server) }
        }
    }

    private func page(plot: JSONValue) throws -> String {
        let value = JSONValue.object([
            "view": .string("movies"), "sort": .string("added"), "total": .number(1),
            "offset": .number(0), "limit": .number(60), "letters": .null,
            "items": .array([.object(["id": .string("sample"), "kind": .string("video"), "title": .string("Sample"), "plot": plot])])
        ])
        return String(decoding: try JSONEncoder().encode(value), as: UTF8.self)
    }
}
