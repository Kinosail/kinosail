import Foundation
import Testing
@testable import KinosailPlayer

struct ReaderOperationTests {
    @Test func rejectsInvalidReaderInputsBeforeNetwork() async throws {
        let fixture = try HTTPFixture(body: "{}")
        defer { fixture.remove() }
        await #expect(throws: ClientError.self) { try await fixture.client.reader(itemID: "../other") }
        for (page, offset) in [(0, 0.0), (10_001, 0), (1, -0.1), (1, 1.1), (1, Double.infinity), (1, Double.nan)] {
            await #expect(throws: ClientError.self) { try await fixture.client.saveReaderPosition(itemID: "book", page: page, offset: offset) }
        }
        #expect(fixture.requests.isEmpty)
    }

    @Test func confinesResourcesToTheBookAndOrigin() throws {
        let server = try ServerAddress("https://media.example")
        #expect(try ReaderResourcePolicy.remote("/read/book/asset/OEBPS/chapter%201.xhtml", itemID: "book", server: server).path.hasSuffix("chapter 1.xhtml"))
        for path in ["/read/other/file", "/media/book", "https://other.example/read/book/file", "/read/book/asset/../file", "/read/book/asset/%2e%2e/file", "/read/book/asset/%2fsecret", "/read/book/file?token=secret", "/read/book/file#fragment", "/read/book/asset/", "/read/book/asset/%0aheader"] {
            #expect(throws: ClientError.self) { try ReaderResourcePolicy.remote(path, itemID: "book", server: server) }
        }
    }

    @Test func requiresSequentialUniquePagesAndBoundedOffsets() throws {
        let server = try ServerAddress("https://media.example")
        let page: JSONValue = .object(["number": .number(1), "title": .string("Document"), "url": .string("/read/book/file")])
        var book: [String: JSONValue] = ["id": .string("book"), "title": .string("Book"), "type": .string("pdf"), "pages": .array([page])]
        #expect(try ReaderBook(.object(book), itemID: "book", server: server).pages.count == 1)
        book["pages"] = .array([page, page])
        #expect(throws: ClientError.self) { try ReaderBook(.object(book), itemID: "book", server: server) }
        for (page, total, offset) in [(0.0, 1.0, 0.0), (2, 1, 0), (1, 0, 0), (1.5, 2, 0), (1, 10_001, 0), (1, 1, 1.1)] {
            #expect(throws: ClientError.self) { try ReaderPosition(.object(["page": .number(page), "total": .number(total), "offset": .number(offset)])) }
        }
    }
}
